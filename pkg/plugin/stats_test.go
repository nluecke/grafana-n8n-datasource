package plugin

import (
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/nluecke/grafana-n8n-datasource/pkg/n8n"
	"github.com/nluecke/grafana-n8n-datasource/pkg/n8n/n8ntest"
	"github.com/stretchr/testify/require"
)

var t0 = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

func TestPercentile(t *testing.T) {
	vals := []float64{15, 20, 35, 40, 50}
	cases := map[float64]float64{0.0001: 15, 50: 35, 90: 46, 99: 49.6, 100: 50}
	for p, want := range cases {
		got, ok := percentile(vals, p)
		require.True(t, ok)
		require.InDelta(t, want, got, 0.01, "p%v", p)
	}
	got, ok := percentile([]float64{7}, 90)
	require.True(t, ok)
	require.Equal(t, 7.0, got)
	_, ok = percentile(nil, 50)
	require.False(t, ok)
}

func TestBuckets(t *testing.T) {
	tr := backend.TimeRange{From: t0.Add(-59*time.Minute - 30*time.Second), To: t0}
	b := newBuckets(tr, 10*time.Minute)
	require.Equal(t, t0.Add(-time.Hour), b.start)
	require.Equal(t, 7, b.n)
	require.Equal(t, 0, b.index(t0.Add(-55*time.Minute)))
	require.Equal(t, 6, b.index(t0))
	require.Equal(t, -1, b.index(t0.Add(-2*time.Hour)))

	// Too fine intervals are widened to stay below maxBuckets.
	b = newBuckets(backend.TimeRange{From: t0.Add(-30 * 24 * time.Hour), To: t0}, time.Second)
	require.LessOrEqual(t, b.n, maxBuckets+1)
}

func statsInput() ([]n8n.Execution, backend.TimeRange) {
	// 60 executions in the last hour, one per minute; every 4th fails; durations 1..60 s.
	execs := n8ntest.Generate(n8ntest.GenOptions{
		Count: 60, Newest: t0.Add(-30 * time.Second), Step: time.Minute,
		Workflows: []string{"wf1", "wf2"},
		Status: func(i int) string {
			if i%4 == 0 {
				return "error"
			}
			return "success"
		},
		Duration: func(i int) time.Duration { return time.Duration(i+1) * time.Second },
	})
	return execs, backend.TimeRange{From: t0.Add(-time.Hour), To: t0}
}

func floatValues(t *testing.T, f *data.Frame) []float64 {
	t.Helper()
	out := make([]float64, 0, f.Rows())
	for i := 0; i < f.Rows(); i++ {
		v, ok := f.Fields[1].ConcreteAt(i)
		if !ok {
			out = append(out, -1)
			continue
		}
		out = append(out, v.(float64))
	}
	return out
}

func sum(vs []float64) float64 {
	s := 0.0
	for _, v := range vs {
		if v > 0 {
			s += v
		}
	}
	return s
}

func TestStatsCountByStatus(t *testing.T) {
	execs, tr := statsInput()
	frames, err := buildStatsFrames(execs, nil, tr, 10*time.Minute, queryModel{Metric: MetricCount})
	require.NoError(t, err)
	require.Len(t, frames, 2)
	byStatus := map[string]float64{}
	for _, f := range frames {
		byStatus[f.Fields[1].Labels["status"]] = sum(floatValues(t, f))
		require.Equal(t, "short", f.Fields[1].Config.Unit)
	}
	require.Equal(t, map[string]float64{"error": 15, "success": 45}, byStatus)
}

func TestStatsCountEmitsZeroSeries(t *testing.T) {
	_, tr := statsInput()
	frames, err := buildStatsFrames(nil, nil, tr, 10*time.Minute, queryModel{Metric: MetricCount, GroupBy: GroupByNone})
	require.NoError(t, err)
	require.Len(t, frames, 1)
	for _, v := range floatValues(t, frames[0]) {
		require.Equal(t, 0.0, v)
	}

	frames, err = buildStatsFrames(nil, nil, tr, 10*time.Minute, queryModel{Metric: MetricCount})
	require.NoError(t, err)
	require.Len(t, frames, 2, "success and error series are always present")
}

func TestStatsRates(t *testing.T) {
	execs, tr := statsInput()
	running := *execs[0].StartedAt
	execs = append(execs, n8n.Execution{ID: "999", Status: "running", StartedAt: &running})

	frames, err := buildStatsFrames(execs, nil, tr, time.Hour, queryModel{Metric: MetricSuccessRate})
	require.NoError(t, err)
	require.Len(t, frames, 1)
	vals := floatValues(t, frames[0])
	require.InDelta(t, 0.75, vals[0], 1e-9, "running executions are ignored")
	require.Equal(t, "percentunit", frames[0].Fields[1].Config.Unit)

	frames, err = buildStatsFrames(execs, nil, tr, time.Hour, queryModel{Metric: MetricErrorRate})
	require.NoError(t, err)
	require.InDelta(t, 0.25, floatValues(t, frames[0])[0], 1e-9)

	// Buckets without finished executions are null, not 0.
	frames, err = buildStatsFrames(nil, nil, tr, time.Hour, queryModel{Metric: MetricSuccessRate})
	require.NoError(t, err)
	v, ok := frames[0].Fields[1].ConcreteAt(0)
	require.False(t, ok, "got %v", v)
}

func TestStatsDurationPercentilesByWorkflow(t *testing.T) {
	execs, tr := statsInput()
	wfs := map[string]n8n.Workflow{"wf1": {ID: "wf1", Name: "Alpha"}, "wf2": {ID: "wf2", Name: "Beta"}}
	frames, err := buildStatsFrames(execs, wfs, tr, time.Hour, queryModel{Metric: MetricDuration, GroupBy: GroupByWorkflow, Percentiles: []float64{50, 90}})
	require.NoError(t, err)
	require.Len(t, frames, 4)
	got := map[string]float64{}
	for _, f := range frames {
		got[f.Fields[1].Config.DisplayNameFromDS] = floatValues(t, f)[0]
		require.Equal(t, "ms", f.Fields[1].Config.Unit)
	}
	// wf1 ran the even executions (durations 1,3,...,59 s), wf2 the odd ones (2,4,...,60 s).
	require.InDelta(t, 30000, got["Alpha p50"], 1)
	require.InDelta(t, 53200, got["Alpha p90"], 1) // rank 0.9*29=26.1 → 53 s + 0.1*2 s
	require.InDelta(t, 31000, got["Beta p50"], 1)
}

func TestStatsValidation(t *testing.T) {
	_, tr := statsInput()
	for _, qm := range []queryModel{
		{Metric: "nope"},
		{Metric: MetricDuration, GroupBy: GroupByStatus},
		{Metric: MetricCount, GroupBy: "nope"},
		{Metric: MetricDuration, Percentiles: []float64{0}},
	} {
		_, err := buildStatsFrames(nil, nil, tr, time.Minute, qm)
		var ue userError
		require.ErrorAs(t, err, &ue, "%+v", qm)
	}
}

func TestStatsRangeBucket(t *testing.T) {
	execs, tr := statsInput()
	frames, err := buildStatsFrames(execs, nil, tr, time.Minute, queryModel{Metric: MetricSuccessRate, Bucket: BucketRange})
	require.NoError(t, err)
	require.Equal(t, 1, frames[0].Rows())
	require.Equal(t, tr.From, frames[0].Fields[0].At(0))
	require.InDelta(t, 0.75, floatValues(t, frames[0])[0], 1e-9)

	frames, err = buildStatsFrames(execs, nil, tr, time.Minute, queryModel{Metric: MetricCount, GroupBy: GroupByNone, Bucket: BucketRange})
	require.NoError(t, err)
	require.Equal(t, []float64{60}, floatValues(t, frames[0]))

	_, err = buildStatsFrames(execs, nil, tr, time.Minute, queryModel{Bucket: "nope"})
	require.Error(t, err)
}
