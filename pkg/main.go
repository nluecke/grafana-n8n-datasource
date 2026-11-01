package main

import (
	"os"

	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/nluecke/grafana-n8n-datasource/pkg/plugin"
)

func main() {
	// Manage serves Grafana's requests until the process exits. It creates one
	// datasource instance per data source configuration via NewDatasource and
	// disposes it when the configuration changes.
	if err := datasource.Manage("nluecke-n8n-datasource", plugin.NewDatasource, datasource.ManageOpts{}); err != nil {
		log.DefaultLogger.Error(err.Error())
		os.Exit(1)
	}
}
