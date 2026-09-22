// Package provideradapter translates provider-specific usage into one display
// contract. Credentials stay in the backend and never appear in Usage.
package provideradapter

import "context"

type Usage struct {
	AccountCount     int      `json:"accountCount,omitempty"`
	Weighting        string   `json:"weighting,omitempty"`
	SyncedAt         int64    `json:"syncedAt,omitempty"`
	Adapter          string   `json:"adapter"`
	Kind             string   `json:"kind"`
	TotalTokens      *int64   `json:"totalTokens,omitempty"`
	RemainingPercent *float64 `json:"remainingPercent,omitempty"`
	ResetsAt         int64    `json:"resetsAt,omitempty"`
	UpdatedAt        int64    `json:"updatedAt,omitempty"`
	Error            string   `json:"error,omitempty"`
}
type Binding struct {
	AccountWeights map[string]float64 `json:"accountWeights,omitempty"`
	Type           string             `json:"type"`
	BaseURL        string             `json:"baseUrl"`
	DatabasePath   string             `json:"databasePath"`
}
type Config struct {
	BaseURL    string
	Credential string
	Binding    Binding
}
type Adapter interface {
	Match(Config) bool
	Read(context.Context, Config) Usage
}

func Resolve(c Config) Adapter {
	for _, adapter := range []Adapter{Grok{}, Kimi{}} {
		if adapter.Match(c) {
			return adapter
		}
	}
	return nil
}
