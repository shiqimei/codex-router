package mux

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shiqimei/codex-router/internal/state"
)

type ModelContext struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	Reasoning       string `json:"reasoning"`
	CatalogRevision string `json:"catalogRevision"`
	HasCatalog      bool   `json:"hasCatalog"`
}

func (m *Multiplexer) ModelContext(thread string) (ModelContext, error) {
	id := ""
	if thread != "" {
		id, _ = m.store.ThreadOwner(thread)
	} else {
		id = m.store.DefaultAccount()
	}
	if id == "" {
		if a, ok := m.store.Controller(); ok {
			id = a.ID
		}
	}
	a, ok := m.store.Account(id)
	if !ok {
		return ModelContext{}, fmt.Errorf("connection unavailable")
	}
	value := ModelContext{ID: a.ID, Kind: a.Kind, Provider: a.Provider, Model: a.Model}
	if value.Model == "" {
		value.Model = m.store.DefaultModel(a)
	}
	if saved := m.store.ThreadModel(thread, a.ID); saved != "" {
		value.Model = saved
	}
	value.Reasoning = m.store.DefaultReasoning(a)
	for _, model := range m.store.Models(a) {
		if model.Slug == value.Model {
			value.Reasoning = model.DefaultReasoning
		}
	}
	if data, err := os.ReadFile(filepath.Join(a.CodexHome, "models.json")); err == nil {
		value.HasCatalog = true
		value.CatalogRevision = fmt.Sprintf("%x", sha256.Sum256(data))
	}
	return value, nil
}
func (m *Multiplexer) providerModel(account state.Account, p map[string]any, method string) string {
	models := m.store.Models(account)
	if len(models) == 0 {
		return account.Model
	}
	allowed := map[string]bool{}
	for _, model := range models {
		allowed[model.Slug] = true
	}
	selected, _ := p["model"].(string)
	if mode, ok := p["collaborationMode"].(map[string]any); ok {
		if settings, ok := mode["settings"].(map[string]any); ok {
			if model, ok := settings["model"].(string); ok && allowed[model] {
				selected = model
			}
		}
	}
	if allowed[selected] {
		return selected
	}
	if thread, ok := p["threadId"].(string); ok {
		saved := m.store.ThreadModel(thread, account.ID)
		if allowed[saved] {
			return saved
		}
	}
	return account.Model
}
func (m *Multiplexer) rememberRequestedModel(thread, owner string, params json.RawMessage) {
	var p struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(params, &p) == nil && p.Model != "" {
		_ = m.store.SetThreadModel(thread, owner, p.Model)
	}
}

// A prior connection/catalog may leave an incompatible effort in native thread
// or collaboration settings even when the picker displays a fallback value.
func (m *Multiplexer) normalizeProviderEffort(account state.Account, slug string, p map[string]any, key string) {
	for _, model := range m.store.Models(account) {
		if model.Slug != slug || len(model.Reasoning) == 0 {
			continue
		}
		allowed := map[string]bool{}
		for _, level := range model.Reasoning {
			allowed[level.Effort] = true
		}
		fallback := model.DefaultReasoning
		if !allowed[fallback] {
			fallback = model.Reasoning[0].Effort
		}
		effort, _ := p[key].(string)
		if !allowed[effort] {
			p[key] = fallback
		}
		if mode, ok := p["collaborationMode"].(map[string]any); ok {
			if settings, ok := mode["settings"].(map[string]any); ok {
				effort, _ := settings["reasoning_effort"].(string)
				if !allowed[effort] {
					settings["reasoning_effort"] = fallback
				}
			}
		}
		return
	}
}
