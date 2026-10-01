// Package sailpoint resolves tenant credentials and owns the shared API client.
package sailpoint

import (
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/go-retryablehttp"
	sdk "github.com/sailpoint-oss/golang-sdk/v3"
)

const setupHint = "Copy .env.example to .env and set SAIL_BASE_URL, SAIL_CLIENT_ID and " +
	"SAIL_CLIENT_SECRET (a personal access token pair from your tenant), or pass them in " +
	"your MCP client's env block. See README.md."

// NewConfiguration resolves credentials from the SAIL_* environment variables
// (populated from .env by LoadDotEnv at startup) and validates that the pieces
// we actually need came back. The SDK would also fall back to ./config.json and
// ~/.sailpoint/config.yaml; this template does not use them.
//
// The SDK resolves partial configs silently (an OAuth-flavoured config.yaml
// yields a base URL but no client credentials), and it panics rather than
// returning an error when nothing is found at all. Both are converted into a
// plain error here so startup can fail with a clear message instead of a stack
// trace, or an opaque HTTP 401 on the first tool call.
func NewConfiguration() (*sdk.Configuration, error) {
	configuration, err := defaultConfiguration()
	if err != nil {
		return nil, fmt.Errorf("unable to resolve SailPoint configuration: %w. %s", err, setupHint)
	}

	client := configuration.ClientConfiguration
	var missing []string
	for _, part := range []struct {
		label string
		value string
	}{
		{"base URL", client.BaseURL},
		{"client ID", client.ClientId},
		{"client secret", client.ClientSecret},
		{"token URL", client.TokenURL},
	} {
		if part.value == "" {
			missing = append(missing, part.label)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("incomplete SailPoint configuration (missing: %s). %s",
			strings.Join(missing, ", "), setupHint)
	}

	// The SDK builds a retryablehttp client with a logger that narrates every
	// request. Supply a quiet one instead; stderr is for our diagnostics.
	httpClient := retryablehttp.NewClient()
	httpClient.Logger = nil
	configuration.HTTPClient = httpClient

	return configuration, nil
}

// defaultConfiguration wraps sdk.NewDefaultConfiguration, which panics when no
// configuration source can be found or a config file is malformed.
func defaultConfiguration() (configuration *sdk.Configuration, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%v", recovered)
		}
	}()
	return sdk.NewDefaultConfiguration(), nil
}

var (
	clientOnce sync.Once
	client     *sdk.APIClient
	clientErr  error
)

// Client lazily builds the shared API client. One client is reused across tool
// calls so the underlying HTTP connection pool is shared.
func Client() (*sdk.APIClient, error) {
	clientOnce.Do(func() {
		configuration, err := NewConfiguration()
		if err != nil {
			clientErr = err
			return
		}
		client = sdk.NewAPIClient(configuration)
	})
	return client, clientErr
}
