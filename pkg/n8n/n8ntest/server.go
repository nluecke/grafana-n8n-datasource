// Package n8ntest provides a fake n8n public API for tests. Its behaviour follows
// the responses recorded in docs/poc-findings.md (n8n 2.42.7 and an older release).
package n8ntest

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/nluecke/grafana-n8n-datasource/pkg/n8n"
)

// Key is the API key the fake server accepts; tests only.
const Key = "fake-api-key"

// Server is a fake n8n instance serving under /api/v1.
type Server struct {
	*httptest.Server

	mu sync.Mutex
	// Executions in any order; served newest (highest id) first.
	Executions []n8n.Execution
	Workflows  []n8n.Workflow
	Tags       []n8n.Tag
	// LegacyAPI mimics older n8n: no startedAfter/startedBefore, limit > 250 rejected.
	LegacyAPI bool
	// ProjectsForbidden mimics a licence without project support.
	ProjectsForbidden bool
	// Insights answers /insights/summary. Nil returns zero values.
	Insights func(start, end time.Time) (int, any)
	// Audit is the POST /audit response body. Nil returns [].
	Audit any
	// FailNext makes the next n requests fail with FailStatus.
	FailNext   int
	FailStatus int
	RetryAfter string

	requests []string
}

// NewServer starts a fake server. Call Close when done.
func NewServer() *Server {
	s := &Server{}
	s.Server = httptest.NewServer(s)
	return s
}

// APIURL returns the API base URL including /api/v1.
func (s *Server) APIURL() string { return s.URL + "/api/v1" }

// Requests returns the request log as "METHOD path?query".
func (s *Server) Requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.requests)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, r.Method+" "+r.URL.RequestURI())

	if s.FailNext > 0 {
		s.FailNext--
		if s.RetryAfter != "" {
			w.Header().Set("Retry-After", s.RetryAfter)
		}
		writeJSON(w, s.FailStatus, map[string]string{"message": http.StatusText(s.FailStatus)})
		return
	}
	if r.Header.Get(n8n.AuthHeader) != Key {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "Unauthorized"})
		return
	}

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/executions":
		s.executions(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/workflows":
		writeJSON(w, http.StatusOK, map[string]any{"data": s.Workflows, "nextCursor": nil})
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tags":
		writeJSON(w, http.StatusOK, map[string]any{"data": s.Tags, "nextCursor": nil})
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects":
		if s.ProjectsForbidden {
			writeJSON(w, http.StatusForbidden, map[string]string{"message": "Your license does not allow for feat:projectRole:admin."})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": []n8n.Project{{ID: "p1", Name: "Default"}}, "nextCursor": nil})
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/insights/summary":
		start, _ := time.Parse(time.RFC3339, r.URL.Query().Get("startDate"))
		end, _ := time.Parse(time.RFC3339, r.URL.Query().Get("endDate"))
		if s.Insights == nil {
			writeJSON(w, http.StatusOK, n8n.InsightsSummary{})
			return
		}
		status, body := s.Insights(start, end)
		writeJSON(w, status, body)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/audit":
		if s.Audit == nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		writeJSON(w, http.StatusOK, s.Audit)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"message": "not found"})
	}
}

func (s *Server) executions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 100
	if v := q.Get("limit"); v != "" {
		limit, _ = strconv.Atoi(v)
	}
	if limit > n8n.MaxPageLimit {
		if s.LegacyAPI {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "request/query/limit must be <= 250"})
			return
		}
		limit = n8n.MaxPageLimit
	}
	if s.LegacyAPI && (q.Has("startedAfter") || q.Has("startedBefore")) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Unknown query parameter 'startedAfter'"})
		return
	}

	var after, before *time.Time
	for key, dst := range map[string]**time.Time{"startedAfter": &after, "startedBefore": &before} {
		if v := q.Get(key); v != "" {
			t, err := time.Parse(time.RFC3339Nano, v)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"message": "request/query/" + key + " Invalid datetime"})
				return
			}
			*dst = &t
		}
	}

	lastID := int64(1<<62 - 1)
	if c := q.Get("cursor"); c != "" {
		raw, _ := base64.StdEncoding.DecodeString(c)
		var cur struct {
			LastID string `json:"lastId"`
		}
		_ = json.Unmarshal(raw, &cur)
		lastID, _ = strconv.ParseInt(cur.LastID, 10, 64)
	}

	sorted := slices.Clone(s.Executions)
	slices.SortFunc(sorted, func(a, b n8n.Execution) int { return int(id(b) - id(a)) })

	var out []n8n.Execution
	more := false
	for _, e := range sorted {
		if id(e) >= lastID {
			continue
		}
		if st := q.Get("status"); st != "" && e.Status != st {
			continue
		}
		if wf := q.Get("workflowId"); wf != "" && e.WorkflowID != wf {
			continue
		}
		if after != nil && (e.StartedAt == nil || e.StartedAt.Before(*after)) {
			continue
		}
		if before != nil && (e.StartedAt == nil || e.StartedAt.After(*before)) {
			continue
		}
		if len(out) == limit {
			more = true
			break
		}
		out = append(out, e)
	}

	var next any
	if more {
		cur, _ := json.Marshal(map[string]any{"lastId": out[len(out)-1].ID, "limit": limit})
		next = base64.StdEncoding.EncodeToString(cur)
	}
	if out == nil {
		out = []n8n.Execution{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out, "nextCursor": next})
}

func id(e n8n.Execution) int64 {
	v, _ := strconv.ParseInt(e.ID, 10, 64)
	return v
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// GenOptions describe synthetic executions.
type GenOptions struct {
	Count     int
	Newest    time.Time
	Step      time.Duration // time between consecutive executions
	Workflows []string
	// Status returns the status of execution i (0 = oldest). Defaults to success.
	Status func(i int) string
	// Duration returns the run time of execution i. Defaults to 100ms.
	Duration func(i int) time.Duration
}

// Generate creates executions with ascending ids and start times.
func Generate(o GenOptions) []n8n.Execution {
	out := make([]n8n.Execution, o.Count)
	oldest := o.Newest.Add(-time.Duration(o.Count-1) * o.Step)
	for i := range out {
		started := oldest.Add(time.Duration(i) * o.Step)
		status := "success"
		if o.Status != nil {
			status = o.Status(i)
		}
		dur := 100 * time.Millisecond
		if o.Duration != nil {
			dur = o.Duration(i)
		}
		stopped := started.Add(dur)
		wf := "wf1"
		if len(o.Workflows) > 0 {
			wf = o.Workflows[i%len(o.Workflows)]
		}
		out[i] = n8n.Execution{
			ID:         fmt.Sprint(i + 1),
			Finished:   status == "success",
			Mode:       "trigger",
			Status:     status,
			StartedAt:  &started,
			StoppedAt:  &stopped,
			WorkflowID: wf,
		}
	}
	return out
}
