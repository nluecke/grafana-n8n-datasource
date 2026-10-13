package plugin

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/nluecke/grafana-n8n-datasource/pkg/models"
	"github.com/nluecke/grafana-n8n-datasource/pkg/n8n"
)

const capabilityRecheck = 10 * time.Minute

// supportsTimeFilter probes once whether n8n accepts startedAfter and caches the answer.
func (d *Datasource) supportsTimeFilter(ctx context.Context) bool {
	d.capMu.Lock()
	defer d.capMu.Unlock()
	if d.capChecked && time.Since(d.capCheckedAt) < capabilityRecheck {
		return d.capTimeFilter
	}
	ok, err := d.client.SupportsStartedAfter(ctx)
	if err != nil {
		// Fall back to client-side filtering; probe again on the next query.
		return false
	}
	d.capChecked, d.capCheckedAt, d.capTimeFilter = true, time.Now(), ok
	return ok
}

// fetchedExecutions is the filtered execution list plus notices for the frame.
type fetchedExecutions struct {
	executions []n8n.Execution
	workflows  map[string]n8n.Workflow
	notices    []data.Notice
}

// fetchExecutions loads executions in the query time range and applies the
// filters n8n cannot apply server-side (multiple statuses or workflows).
func (d *Datasource) fetchExecutions(ctx context.Context, tr backend.TimeRange, qm queryModel) (fetchedExecutions, error) {
	var out fetchedExecutions

	workflows, err := d.workflowMap(ctx)
	if err != nil {
		return out, err
	}
	out.workflows = workflows

	from, to := tr.From, tr.To
	// Align the requested window to the cache TTL so that repeated dashboard
	// refreshes produce identical request URLs. Exact bounds are applied below.
	if ttl := d.cacheTTL(); ttl > 0 {
		from = from.Truncate(ttl)
		to = to.Truncate(ttl).Add(ttl)
	}

	base := n8n.ExecutionFilter{
		ProjectID:        qm.ProjectID,
		From:             from,
		To:               to,
		ServerTimeFilter: d.supportsTimeFilter(ctx),
		MaxPages:         d.settings.MaxPages,
	}

	res, err := d.listExecutionsFanOut(ctx, base, qm)
	if err != nil {
		return out, err
	}

	for _, e := range res.Executions {
		if e.StartedAt != nil && (e.StartedAt.Before(tr.From) || e.StartedAt.After(tr.To)) {
			continue
		}
		if len(qm.Statuses) > 0 && !slices.Contains(qm.Statuses, e.Status) {
			continue
		}
		if len(qm.WorkflowIDs) > 0 && !slices.Contains(qm.WorkflowIDs, e.WorkflowID) {
			continue
		}
		out.executions = append(out.executions, e)
	}

	if res.Truncated {
		out.notices = append(out.notices, d.truncationNotice(res, tr))
	} else if notice, ok := d.historyNotice(ctx, base, res.ExecutionResult, tr); ok {
		out.notices = append(out.notices, notice)
	}

	if notice, ok := skippedSuccessNotice(workflows, qm.WorkflowIDs, out.executions); ok {
		out.notices = append(out.notices, notice)
	}
	return out, nil
}

// maxFanOut bounds the number of filtered requests per query.
const maxFanOut = 20

// fanOutResult merges the results of several filtered execution lists.
type fanOutResult struct {
	n8n.ExecutionResult
	// CoveredFrom is the start of the window that is complete in all truncated lists.
	CoveredFrom *time.Time
	// TruncatedPages is the page count of the list that limits CoveredFrom.
	TruncatedPages int
}

// listExecutionsFanOut sends one request per selected status and workflow, because
// n8n filters by a single status and workflow only. Filtering on the server keeps
// filtered panels (e.g. failed executions) complete on instances with many executions.
// Above maxFanOut combinations it falls back to filtering in the plugin.
func (d *Datasource) listExecutionsFanOut(ctx context.Context, base n8n.ExecutionFilter, qm queryModel) (fanOutResult, error) {
	statuses, workflows := qm.Statuses, qm.WorkflowIDs
	if len(statuses) == 0 {
		statuses = []string{""}
	}
	if len(workflows) == 0 {
		workflows = []string{""}
	}
	if len(statuses)*len(workflows) > maxFanOut {
		workflows = []string{""}
		if len(statuses) > maxFanOut {
			statuses = []string{""}
		}
	}

	var filters []n8n.ExecutionFilter
	for _, st := range statuses {
		for _, wf := range workflows {
			f := base
			f.Status, f.WorkflowID = st, wf
			filters = append(filters, f)
		}
	}

	results := make([]n8n.ExecutionResult, len(filters))
	errs := make([]error, len(filters))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i, f := range filters {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			results[i], errs[i] = d.client.ListExecutions(ctx, f)
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return fanOutResult{}, err
	}

	var out fanOutResult
	seen := map[string]bool{}
	for _, r := range results {
		out.Pages += r.Pages
		if r.Oldest != nil && (out.Oldest == nil || r.Oldest.Before(*out.Oldest)) {
			out.Oldest = r.Oldest
		}
		if r.Truncated {
			out.Truncated = true
			if r.Oldest != nil && (out.CoveredFrom == nil || r.Oldest.After(*out.CoveredFrom)) {
				out.CoveredFrom, out.TruncatedPages = r.Oldest, r.Pages
			}
		}
		for _, e := range r.Executions {
			if !seen[e.ID] {
				seen[e.ID] = true
				out.Executions = append(out.Executions, e)
			}
		}
	}
	if len(results) > 1 {
		slices.SortStableFunc(out.Executions, func(a, b n8n.Execution) int {
			switch {
			case a.StartedAt == nil && b.StartedAt == nil:
				return 0
			case a.StartedAt == nil:
				return -1
			case b.StartedAt == nil:
				return 1
			}
			return b.StartedAt.Compare(*a.StartedAt)
		})
	}
	return out, nil
}

// truncationNotice explains an incomplete result and how to fix it. The panel
// icon links to the data source settings. n8n offers no execution count, so the
// page estimate assumes an even execution rate and is labelled as such.
func (d *Datasource) truncationNotice(res fanOutResult, tr backend.TimeRange) data.Notice {
	maxPages := d.settings.MaxPages
	fix := fmt.Sprintf(" Fix: raise \"Max pages\" in the data source settings (now %d; each page adds %d executions, maximum %d), "+
		"or narrow the time range or filters.", maxPages, n8n.MaxPageLimit, models.MaxMaxPages)
	text := fmt.Sprintf("Incomplete data: this time range holds more executions than one query may load (%s).",
		thousands(maxPages*n8n.MaxPageLimit))
	if res.CoveredFrom != nil && res.CoveredFrom.After(tr.From) {
		rangeLen, covered := tr.To.Sub(tr.From), tr.To.Sub(*res.CoveredFrom)
		text = fmt.Sprintf("Incomplete data: only the newest %s executions were loaded, covering %s to %s UTC (%.0f%% of the time range).",
			thousands(res.TruncatedPages*n8n.MaxPageLimit), res.CoveredFrom.UTC().Format("2006-01-02 15:04"),
			tr.To.UTC().Format("15:04"), 100*covered.Seconds()/rangeLen.Seconds())
		text += fix
		if covered > 0 {
			needed := int(math.Ceil(float64(res.TruncatedPages) * rangeLen.Seconds() / covered.Seconds()))
			if needed <= models.MaxMaxPages {
				text += fmt.Sprintf(" At an even execution rate about %d pages would cover the full range.", needed)
			} else {
				text += fmt.Sprintf(" At an even execution rate the full range would need about %d pages, more than the maximum: "+
					"use a shorter range, or the Insights query for totals.", needed)
			}
		}
	} else {
		text += fix
	}
	n := data.Notice{Severity: data.NoticeSeverityWarning, Text: text}
	if d.uid != "" {
		n.Link = "/connections/datasources/edit/" + d.uid
	}
	return n
}

// thousands formats n with comma separators, e.g. 5,000.
func thousands(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// historyNotice reports when n8n holds no executions for the beginning of the
// time range, usually because of execution pruning or plan retention.
func (d *Datasource) historyNotice(ctx context.Context, f n8n.ExecutionFilter, res n8n.ExecutionResult, tr backend.TimeRange) (data.Notice, bool) {
	rangeLen := tr.To.Sub(tr.From)
	if res.Oldest != nil && res.Oldest.Sub(tr.From) < rangeLen/20 {
		return data.Notice{}, false
	}
	if f.ServerTimeFilter {
		// The server filter hides older executions; check whether any exist.
		var older struct {
			Data []n8n.Execution `json:"data"`
		}
		q := url.Values{"limit": {"1"}, "startedBefore": {tr.From.UTC().Format(time.RFC3339Nano)}}
		if f.WorkflowID != "" {
			q.Set("workflowId", f.WorkflowID)
		}
		if err := d.client.Get(ctx, "/executions", q, &older); err != nil || len(older.Data) > 0 {
			return data.Notice{}, false
		}
	} else if res.Oldest != nil && res.Oldest.Before(tr.From) {
		return data.Notice{}, false
	}
	text := "n8n holds no executions before the start of the time range."
	if res.Oldest != nil {
		text = fmt.Sprintf("n8n holds no executions before %s.", res.Oldest.UTC().Format(time.RFC3339))
	}
	return data.Notice{
		Severity: data.NoticeSeverityInfo,
		Text: text + " Older executions were pruned (EXECUTIONS_DATA_MAX_AGE / EXECUTIONS_DATA_PRUNE_MAX_COUNT, " +
			"or the retention of the n8n Cloud plan) or never ran. Use the Insights query for longer ranges.",
	}, true
}

// skippedSuccessNotice warns when workflows in scope do not store successful
// executions, which makes counts and success rates from the execution list wrong.
func skippedSuccessNotice(workflows map[string]n8n.Workflow, filter []string, execs []n8n.Execution) (data.Notice, bool) {
	inScope := map[string]bool{}
	if len(filter) > 0 {
		for _, id := range filter {
			inScope[id] = true
		}
	} else {
		for _, e := range execs {
			inScope[e.WorkflowID] = true
		}
	}
	var names []string
	for id := range inScope {
		if wf, ok := workflows[id]; ok && wf.Settings.SaveDataSuccessExecution == "none" {
			names = append(names, wf.Name)
		}
	}
	if len(names) == 0 {
		return data.Notice{}, false
	}
	slices.Sort(names)
	return data.Notice{
		Severity: data.NoticeSeverityWarning,
		Text: "These workflows do not save successful executions, so counts and success rates are too low: " +
			strings.Join(names, ", ") + ". The instance setting EXECUTIONS_DATA_SAVE_ON_SUCCESS=none has the same effect and cannot be detected via the API.",
	}, true
}

func (d *Datasource) workflowMap(ctx context.Context) (map[string]n8n.Workflow, error) {
	wfs, err := d.client.ListWorkflows(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[string]n8n.Workflow, len(wfs))
	for _, w := range wfs {
		m[w.ID] = w
	}
	return m, nil
}

func workflowName(workflows map[string]n8n.Workflow, id string) string {
	if wf, ok := workflows[id]; ok {
		return wf.Name
	}
	return id
}
