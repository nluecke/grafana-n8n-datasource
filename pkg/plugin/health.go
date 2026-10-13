package plugin

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/nluecke/grafana-n8n-datasource/pkg/n8n"
)

const healthTimeout = 15 * time.Second

// CheckHealth verifies URL, API key and API availability with GET /workflows?limit=1.
func (d *Datasource) CheckHealth(ctx context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if d.settingsErr != nil {
		return healthError(d.settingsErr.Error()), nil
	}

	ctx, cancel := context.WithTimeout(n8n.WithoutCache(ctx), healthTimeout)
	defer cancel()

	err := d.client.Get(ctx, "/workflows", url.Values{"limit": {"1"}}, nil)
	if err != nil {
		return healthError(describeHealthError(err, d.settings.URL, d.settings.EnableSecureSocksProxy)), nil
	}

	msg := "Connected to the n8n API."
	if supported, err := d.client.SupportsStartedAfter(ctx); err == nil {
		if supported {
			msg += " Server-side time filter available."
		} else {
			msg += " This n8n version has no server-side time filter; executions are paged until the start of the time range."
		}
	}
	return &backend.CheckHealthResult{Status: backend.HealthStatusOk, Message: msg}, nil
}

func apiPathHint(baseURL string) string {
	base := strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(base, "/api/v1") {
		return "Check that the public API is enabled and N8N_PUBLIC_API_ENDPOINT is not changed."
	}
	return fmt.Sprintf("Try %s/api/v1.", base)
}

func healthError(msg string) *backend.CheckHealthResult {
	return &backend.CheckHealthResult{Status: backend.HealthStatusError, Message: msg}
}

func describeHealthError(err error, baseURL string, viaPDC bool) string {
	var apiErr *n8n.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusUnauthorized:
			return "n8n rejected the API key (401). Check that the key is valid and not expired. " +
				"n8n answers the same way when the public API is disabled (N8N_PUBLIC_API_DISABLED=true)."
		case http.StatusForbidden:
			return "n8n denied access (403). The public API is not available in the n8n Cloud free trial, " +
				"or the API key lacks the workflow:list scope (n8n enforces key scopes)."
		case http.StatusNotFound:
			// n8n answers unknown paths with 404 JSON when asked for JSON (HTML otherwise).
			return "n8n API not found (404). The URL must point to the public API and end with /api/v1. " + apiPathHint(baseURL)
		case http.StatusTooManyRequests:
			return "n8n is rate limiting requests (429). Try again later."
		}
		return fmt.Sprintf("Unexpected response from n8n: %s", apiErr.Error())
	}

	if errors.Is(err, n8n.ErrNotJSON) {
		return "The URL answered with a web page instead of the n8n API. " + apiPathHint(baseURL)
	}

	if errors.Is(err, n8n.ErrNotAllowed) {
		return err.Error()
	}

	var unknownAuth x509.UnknownAuthorityError
	var certInvalid x509.CertificateInvalidError
	var hostErr x509.HostnameError
	if errors.As(err, &unknownAuth) || errors.As(err, &certInvalid) || errors.As(err, &hostErr) ||
		strings.Contains(err.Error(), "x509:") || strings.Contains(err.Error(), "tls:") {
		return "TLS error while connecting to n8n: " + err.Error() +
			". For an internal CA enable \"With CA cert\" and paste the CA certificate."
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return "Timed out connecting to n8n. Check the URL and that Grafana can reach the host."
	}

	var netErr net.Error
	var urlErr *url.Error
	if viaPDC && (errors.As(err, &netErr) || errors.As(err, &urlErr)) {
		return "Cannot reach n8n through Private Data Source Connect: " + err.Error() +
			". Check that the PDC agent is running and can reach the n8n URL from its network."
	}
	if errors.As(err, &netErr) || errors.As(err, &urlErr) {
		return "Cannot reach n8n: " + err.Error() +
			". Grafana Cloud cannot reach hosts on private networks; use a public URL or Private Data Source Connect."
	}

	return "Unexpected error: " + err.Error()
}
