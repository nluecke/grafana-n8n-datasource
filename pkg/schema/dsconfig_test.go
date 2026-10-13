package schema_test

import (
	_ "embed"
	"testing"

	"github.com/grafana/dsconfig/schema"
	"github.com/nluecke/grafana-n8n-datasource/pkg/models"
)

//go:embed dsconfig.json
var configSchemaJSON []byte

//go:generate go test -run TestPlugin -generateArtifacts
func TestPlugin(t *testing.T) {
	schema.RunPluginTests(t, schema.PluginUnderTest{
		ID:                "nluecke-n8n-datasource",
		ConfigSchemaJSON:  configSchemaJSON,
		SettingsJSONModel: models.PluginSettings{},
		SecureKeys:        models.SecureKeys,
	})
}
