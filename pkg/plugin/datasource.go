package plugin

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
	"github.com/nluecke/grafana-n8n-datasource/pkg/models"
	"github.com/nluecke/grafana-n8n-datasource/pkg/n8n"
)

var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ backend.CallResourceHandler   = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

// queryTimeout bounds a single query including all pages and retries.
const queryTimeout = 60 * time.Second

// NewDatasource creates a datasource instance for one configured n8n connection.
// Invalid settings do not fail instance creation so that CheckHealth can report them.
func NewDatasource(ctx context.Context, s backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	settings, err := models.LoadPluginSettings(s)
	if err != nil {
		return nil, err
	}
	ds := &Datasource{settings: settings}
	ds.resourceHandler = httpadapter.New(ds.newResourceMux())
	if ds.settingsErr = settings.Validate(); ds.settingsErr != nil {
		return ds, nil
	}

	opts, err := s.HTTPClientOptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("http client options: %w", err)
	}
	httpClient, err := httpclient.New(opts)
	if err != nil {
		return nil, fmt.Errorf("http client: %w", err)
	}
	ds.httpClient = httpClient
	ds.client = n8n.NewClient(settings.URL, settings.Secrets.ApiKey, httpClient, n8n.Options{
		CacheTTL:   ds.cacheTTL(),
		MaxRetries: 3,
	})
	return ds, nil
}

// Datasource serves queries, resources and health checks for one n8n instance.
type Datasource struct {
	settings        *models.PluginSettings
	settingsErr     error
	httpClient      *http.Client
	client          *n8n.Client
	resourceHandler backend.CallResourceHandler

	capMu         sync.Mutex
	capChecked    bool
	capCheckedAt  time.Time
	capTimeFilter bool
}

// Dispose releases resources when the datasource settings change.
func (d *Datasource) Dispose() {
	if d.httpClient != nil {
		d.httpClient.CloseIdleConnections()
	}
}

// QueryData handles multiple queries and returns multiple responses.
func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	response := backend.NewQueryDataResponse()
	for _, q := range req.Queries {
		response.Responses[q.RefID] = d.query(ctx, q)
	}
	return response, nil
}

// CallResource serves the endpoints used by the query and variable editors.
func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	return d.resourceHandler.CallResource(ctx, req, sender)
}
