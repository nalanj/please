// Package anthropic provides an [llm.Provider] for gateways that expose the
// Anthropic Messages API wire format. Currently supported: MiniMax.
package anthropic

import "net/http"

const (
	miniMaxBaseURL = "https://api.minimax.io/anthropic"
)

// config holds the mutable settings for a [Client].
type config struct {
	baseURL    string
	httpClient *http.Client
}

// Option is a functional option for configuring a [Client].
type Option func(*config)

// WithBaseURL overrides the API base URL. Useful when pointing at a self-hosted
// proxy or a gateway that is not one of the named providers.
func WithBaseURL(url string) Option {
	return func(c *config) { c.baseURL = url }
}

// WithHTTPClient replaces the default HTTP client used for requests.
func WithHTTPClient(client *http.Client) Option {
	return func(c *config) { c.httpClient = client }
}