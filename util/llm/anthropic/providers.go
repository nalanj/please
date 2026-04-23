package anthropic

import "github.com/nalanj/please/util/llm"

// NewMiniMaxProvider returns an [llm.Provider] that sends requests to the
// MiniMax API using the Anthropic Messages API format.
//
// The apiKey is your MiniMax API key.
func NewMiniMaxProvider(apiKey string, opts ...Option) llm.Provider {
	return newClient(apiKey, miniMaxBaseURL, opts...)
}