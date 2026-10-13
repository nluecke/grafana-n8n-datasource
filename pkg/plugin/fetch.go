package plugin

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
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

	f := n8n.ExecutionFilter{
		ProjectID:        qm.ProjectID,
		From:             from,
		To:               to,
		ServerTimeFilter: d.supportsTimeFilter(ctx),
		MaxPages:         d.settings.MaxPages,
	}
	if len(qm.Statuses) == 1 {
		f.Status = qm.Statuses[0]
	}
	if len(qm.WorkflowIDs) == 1 {
		f.WorkflowID = qm.WorkflowIDs[0]
	}

	res, err := d.client.ListExecutions(ctx, f)
	if err != nil {
		return out, err
	}

	for _, e := range res.Executions {
		if e.StartedAt != nil && (e.StartedAt.Before(tr.From) || e.StartedAt.After(tr.To)) {
			continue
		}
		if len(qm.Statuses) > 1 && !slices.Contains(qm.Statuses, e.Status) {
			continue
		}
		if len(qm.WorkflowIDs) > 1 && !slices.Contains(qm.WorkflowIDs, e.WorkflowID) {
			continue
		}
		out.executions = append(out.executions, e)
	}

	if res.Truncated {
		out.notices = append(out.notices, data.Notice{
			Severity: data.NoticeSeverityWarning,
			Text: fmt.Sprintf("Result truncated: stopped after %d pages (%d executions) before reaching the start of the time range. "+
				"Narrow the filters or the time range, or raise \"Max pages\" in the data source settings.", res.Pages, res.Pages*n8n.MaxPageLimit),
		})
	} else if notice, ok := d.historyNotice(ctx, f, res, tr); ok {
		out.notices = append(out.notices, notice)
	}

	if notice, ok := skippedSuccessNotice(workflows, qm.WorkflowIDs, out.executions); ok {
		out.notices = append(out.notices, notice)
	}
	return out, nil
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
