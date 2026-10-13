package plugin

import (
	"context"
	"fmt"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

func (d *Datasource) queryExecutions(ctx context.Context, q backend.DataQuery, qm queryModel) (data.Frames, error) {
	fetched, err := d.fetchExecutions(ctx, q.TimeRange, qm)
	if err != nil {
		return nil, err
	}

	n := len(fetched.executions)
	var (
		ids         = make([]string, n)
		wfIDs       = make([]string, n)
		wfNames     = make([]string, n)
		statuses    = make([]string, n)
		modes       = make([]string, n)
		startedAt   = make([]*time.Time, n)
		stoppedAt   = make([]*time.Time, n)
		durationsMs = make([]*float64, n)
		retryOf     = make([]*string, n)
		links       = make([]string, n)
	)
	base := d.uiBaseURL()
	for i, e := range fetched.executions {
		ids[i] = e.ID
		wfIDs[i] = e.WorkflowID
		wfNames[i] = workflowName(fetched.workflows, e.WorkflowID)
		statuses[i] = e.Status
		modes[i] = e.Mode
		startedAt[i] = e.StartedAt
		stoppedAt[i] = e.StoppedAt
		if dur, ok := e.Duration(); ok {
			ms := float64(dur.Milliseconds())
			durationsMs[i] = &ms
		}
		retryOf[i] = e.RetryOf
		links[i] = fmt.Sprintf("%s/workflow/%s/executions/%s", base, e.WorkflowID, e.ID)
	}

	durationField := data.NewField("durationMs", nil, durationsMs)
	durationField.Config = &data.FieldConfig{DisplayName: "Duration", Unit: "ms"}
	idField := data.NewField("id", nil, ids)
	idField.Config = &data.FieldConfig{
		DisplayName: "Execution",
		Links:       []data.DataLink{{Title: "Open in n8n", URL: "${__data.fields.url}", TargetBlank: true}},
	}

	frame := data.NewFrame("executions",
		idField,
		data.NewField("workflowId", nil, wfIDs),
		data.NewField("workflow", nil, wfNames),
		data.NewField("status", nil, statuses),
		data.NewField("mode", nil, modes),
		data.NewField("startedAt", nil, startedAt),
		data.NewField("stoppedAt", nil, stoppedAt),
		durationField,
		data.NewField("retryOf", nil, retryOf),
		data.NewField("url", nil, links),
	)
	frame.Meta = &data.FrameMeta{
		Type:                   data.FrameTypeTable,
		PreferredVisualization: data.VisTypeTable,
		Notices:                fetched.notices,
	}
	return data.Frames{frame}, nil
}
