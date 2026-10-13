package plugin

import (
	"context"
	"sort"

	"github.com/grafana/grafana-plugin-sdk-go/data"
)

func (d *Datasource) queryAudit(ctx context.Context) (data.Frames, error) {
	reports, err := d.client.Audit(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(reports))
	for name := range reports {
		names = append(names, name)
	}
	sort.Strings(names)

	var report, risk, section, description, recommendation []string
	var locations []int64
	for _, name := range names {
		r := reports[name]
		for _, s := range r.Sections {
			report = append(report, name)
			risk = append(risk, r.Risk)
			section = append(section, s.Title)
			description = append(description, s.Description)
			recommendation = append(recommendation, s.Recommendation)
			locations = append(locations, int64(len(s.Location)))
		}
	}
	frame := data.NewFrame("audit",
		data.NewField("report", nil, report),
		data.NewField("risk", nil, risk),
		data.NewField("section", nil, section),
		data.NewField("description", nil, description),
		data.NewField("recommendation", nil, recommendation),
		data.NewField("locations", nil, locations),
	)
	frame.Meta = &data.FrameMeta{Type: data.FrameTypeTable, PreferredVisualization: data.VisTypeTable}
	return data.Frames{frame}, nil
}
