package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

const (
	DefaultMaxPages        = 20
	DefaultCacheTTLSeconds = 60
	MaxMaxPages            = 100
)

// PluginSettings mirrors jsonData as declared in pkg/schema/dsconfig.json.
// Root and secure values are populated by LoadPluginSettings.
type PluginSettings struct {
	MaxPages          int  `json:"maxPages"`
	CacheTTLSeconds   *int `json:"cacheTtlSeconds"`
	TLSSkipVerify     bool `json:"tlsSkipVerify"`
	TLSAuthWithCACert bool `json:"tlsAuthWithCACert"`
	// Read by the SDK HTTP client (Private Data Source Connect).
	EnableSecureSocksProxy bool `json:"enableSecureSocksProxy"`

	URL     string                `json:"-"`
	Secrets *SecretPluginSettings `json:"-"`
}

type SecretPluginSettings struct {
	ApiKey    string `json:"apiKey"`
	TLSCACert string `json:"tlsCACert"`
}

// SecureKeys lists the secureJsonData keys the plugin reads.
var SecureKeys = []string{"apiKey", "tlsCACert"}

func LoadPluginSettings(source backend.DataSourceInstanceSettings) (*PluginSettings, error) {
	settings := PluginSettings{}
	if len(source.JSONData) > 0 {
		if err := json.Unmarshal(source.JSONData, &settings); err != nil {
			return nil, fmt.Errorf("could not unmarshal PluginSettings json: %w", err)
		}
	}

	if settings.MaxPages <= 0 {
		settings.MaxPages = DefaultMaxPages
	}
	if settings.MaxPages > MaxMaxPages {
		settings.MaxPages = MaxMaxPages
	}
	if settings.CacheTTLSeconds == nil {
		ttl := DefaultCacheTTLSeconds
		settings.CacheTTLSeconds = &ttl
	}

	settings.URL = strings.TrimRight(strings.TrimSpace(source.URL), "/")
	settings.Secrets = loadSecretPluginSettings(source.DecryptedSecureJSONData)

	return &settings, nil
}

// Validate checks the settings required to talk to the n8n API.
func (s *PluginSettings) Validate() error {
	if s.URL == "" {
		return errors.New("URL is missing")
	}
	u, err := url.Parse(s.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("URL must start with http:// or https://")
	}
	if s.Secrets == nil || s.Secrets.ApiKey == "" {
		return errors.New("API key is missing")
	}
	if s.TLSAuthWithCACert && s.Secrets.TLSCACert == "" {
		return errors.New("CA certificate is missing")
	}
	return nil
}

func loadSecretPluginSettings(source map[string]string) *SecretPluginSettings {
	return &SecretPluginSettings{
		ApiKey:    source["apiKey"],
		TLSCACert: source["tlsCACert"],
	}
}
