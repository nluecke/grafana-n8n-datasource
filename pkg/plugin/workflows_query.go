package plugin

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/nluecke/grafana-n8n-datasource/pkg/n8n"
)

func (d *Datasource) queryWorkflows(ctx context.Context, qm queryModel) (data.Frames, error) {
	wfs, err := d.client.ListWorkflows(ctx)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(wfs, func(a, b n8n.Workflow) int { return strings.Compare(a.Name, b.Name) })

	var (
		ids, names, tags, projects, links []string
		active, archived, skipsSuccess    []bool
		triggers                          []int64
		created, updated                  []time.Time
	)
	base := d.uiBaseURL()
	for _, wf := range wfs {
		if len(qm.WorkflowIDs) > 0 && !slices.Contains(qm.WorkflowIDs, wf.ID) {
			continue
		}
		if qm.ProjectID != "" && wf.ProjectID() != qm.ProjectID {
			continue
		}
		tagNames := make([]string, 0, len(wf.Tags))
		for _, t := range wf.Tags {
			tagNames = append(tagNames, t.Name)
		}
		ids = append(ids, wf.ID)
		names = append(names, wf.Name)
		active = append(active, wf.Active)
		archived = append(archived, wf.IsArchived)
		tags = append(tags, strings.Join(tagNames, ", "))
		projects = append(projects, wf.ProjectID())
		triggers = append(triggers, int64(wf.TriggerCount))
		skipsSuccess = append(skipsSuccess, wf.Settings.SaveDataSuccessExecution == "none")
		created = append(created, wf.CreatedAt)
		updated = append(updated, wf.UpdatedAt)
		links = append(links, fmt.Sprintf("%s/workflow/%s", base, wf.ID))
	}

	nameField := data.NewField("name", nil, names)
	nameField.Config = &data.FieldConfig{
		DisplayName: "Workflow",
		Links:       []data.DataLink{{Title: "Open in n8n", URL: "${__data.fields.url}", TargetBlank: true}},
	}
	frame := data.NewFrame("workflows",
		data.NewField("id", nil, ids),
		nameField,
		data.NewField("active", nil, active),
		data.NewField("archived", nil, archived),
		data.NewField("tags", nil, tags),
		data.NewField("projectId", nil, projects),
		data.NewField("triggerCount", nil, triggers),
		data.NewField("skipsSuccessfulExecutions", nil, skipsSuccess),
		data.NewField("createdAt", nil, created),
		data.NewField("updatedAt", nil, updated),
		data.NewField("url", nil, links),
	)
	frame.Meta = &data.FrameMeta{Type: data.FrameTypeTable, PreferredVisualization: data.VisTypeTable}
	return data.Frames{frame}, nil
}
