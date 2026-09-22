package mux

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/pelletier/go-toml/v2"
	"github.com/shiqimei/codex-router/internal/provideradapter"
	"github.com/shiqimei/codex-router/internal/state"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type usageCacheEntry struct {
	Usage   provideradapter.Usage
	Expires time.Time
	Pending chan struct{}
}

func (m *Multiplexer) providerUsage(ctx context.Context, account state.Account) *provideradapter.Usage {
	data, err := os.ReadFile(filepath.Join(account.CodexHome, "provider.toml"))
	if err != nil {
		return nil
	}
	var doc struct {
		Providers map[string]struct {
			BaseURL    string            `toml:"base_url"`
			EnvKey     string            `toml:"env_key"`
			Bearer     string            `toml:"experimental_bearer_token"`
			Headers    map[string]string `toml:"http_headers"`
			EnvHeaders map[string]string `toml:"env_http_headers"`
		} `toml:"model_providers"`
	}
	if toml.Unmarshal(data, &doc) != nil {
		return nil
	}
	p, ok := doc.Providers[account.Provider]
	if !ok {
		return nil
	}
	env := map[string]string{}
	for _, entry := range m.environment {
		if k, v, ok := strings.Cut(entry, "="); ok {
			env[k] = v
		}
	}
	if private, err := os.ReadFile(filepath.Join(account.CodexHome, "provider-env.json")); err == nil {
		var overrides map[string]string
		if json.Unmarshal(private, &overrides) == nil {
			for k, v := range overrides {
				env[k] = v
			}
		}
	}
	c := provideradapter.Config{BaseURL: p.BaseURL, Credential: p.Bearer}
	if p.EnvKey != "" {
		c.Credential = env[p.EnvKey]
	}
	if c.Credential == "" {
		for k, v := range p.Headers {
			if strings.EqualFold(k, "Authorization") {
				c.Credential = strings.TrimPrefix(v, "Bearer ")
			}
		}
		for k, v := range p.EnvHeaders {
			if strings.EqualFold(k, "Authorization") {
				c.Credential = strings.TrimPrefix(env[v], "Bearer ")
			}
		}
	}
	var bindings map[string]provideradapter.Binding
	if data, err := os.ReadFile(filepath.Join(m.store.Root(), "provider-metrics.json")); err == nil {
		_ = json.Unmarshal(data, &bindings)
	}
	c.Binding = bindings[account.ID]
	adapter := provideradapter.Resolve(c)
	if adapter == nil {
		return nil
	}
	encoded, _ := json.Marshal(c)
	key := account.ID + fmt.Sprintf("%x", sha256.Sum256(encoded))
	m.usageMu.Lock()
	if cached, ok := m.usageCache[key]; ok {
		if cached.Pending != nil {
			done := cached.Pending
			m.usageMu.Unlock()
			select {
			case <-done:
				return m.providerUsage(ctx, account)
			case <-ctx.Done():
				return &provideradapter.Usage{Error: "Usage unavailable"}
			}
		}
		if time.Now().Before(cached.Expires) {
			m.usageMu.Unlock()
			value := cached.Usage
			return &value
		}
	}
	if m.usageCache == nil {
		m.usageCache = map[string]usageCacheEntry{}
	}
	done := make(chan struct{})
	m.usageCache[key] = usageCacheEntry{Pending: done}
	m.usageMu.Unlock()
	usage := adapter.Read(ctx, c)
	ttl := 30 * time.Second
	if usage.Error != "" {
		ttl = 5 * time.Second
	}
	m.usageMu.Lock()
	m.usageCache[key] = usageCacheEntry{Usage: usage, Expires: time.Now().Add(ttl)}
	close(done)
	m.usageMu.Unlock()
	return &usage
}
