package plugin

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"github.com/nluecke/grafana-n8n-datasource/pkg/n8n"
)

// option is the value/label shape the frontend uses for selects and variables.
type option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

func (d *Datasource) newResourceMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /workflows", d.handleWorkflows)
	mux.HandleFunc("GET /tags", d.handleTags)
	mux.HandleFunc("GET /projects", d.handleProjects)
	return mux
}

func (d *Datasource) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	if !d.ready(w) {
		return
	}
	wfs, err := d.client.ListWorkflows(r.Context())
	if err != nil {
		writeResourceError(w, err)
		return
	}
	opts := make([]option, 0, len(wfs))
	for _, wf := range wfs {
		if wf.IsArchived {
			continue
		}
		opts = append(opts, option{Value: wf.ID, Label: wf.Name})
	}
	writeOptions(w, opts)
}

func (d *Datasource) handleTags(w http.ResponseWriter, r *http.Request) {
	if !d.ready(w) {
		return
	}
	tags, err := d.client.ListTags(r.Context())
	if err != nil {
		writeResourceError(w, err)
		return
	}
	opts := make([]option, 0, len(tags))
	for _, t := range tags {
		opts = append(opts, option{Value: t.ID, Label: t.Name})
	}
	writeOptions(w, opts)
}

// handleProjects returns an empty list where the licence has no project support.
func (d *Datasource) handleProjects(w http.ResponseWriter, r *http.Request) {
	if !d.ready(w) {
		return
	}
	projects, err := d.client.ListProjects(r.Context())
	var apiErr *n8n.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden {
		writeOptions(w, []option{})
		return
	}
	if err != nil {
		writeResourceError(w, err)
		return
	}
	opts := make([]option, 0, len(projects))
	for _, p := range projects {
		opts = append(opts, option{Value: p.ID, Label: p.Name})
	}
	writeOptions(w, opts)
}

func (d *Datasource) ready(w http.ResponseWriter) bool {
	if d.settingsErr != nil {
		http.Error(w, d.settingsErr.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

func writeOptions(w http.ResponseWriter, opts []option) {
	sort.SliceStable(opts, func(i, j int) bool { return opts[i].Label < opts[j].Label })
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(opts)
}

func writeResourceError(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	var apiErr *n8n.APIError
	if errors.As(err, &apiErr) {
		status = apiErr.StatusCode
	}
	http.Error(w, err.Error(), status)
}
