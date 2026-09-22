package mux

import (
	"encoding/json"
	"github.com/shiqimei/codex-router/internal/protocol"
	"github.com/shiqimei/codex-router/internal/state"
	"testing"
)

func TestProviderCatalogRepairsStaleEffortAndPreservesXHigh(t *testing.T) {
	s, err := state.Open(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	catalog := `{"models":[{"slug":"grok-4.7","default_reasoning_level":"medium","supported_reasoning_levels":[{"effort":"low"},{"effort":"medium"},{"effort":"high"},{"effort":"xhigh"}]}]}`
	a, err := s.AddProvider(state.ProviderInput{Label: "Grok", ConfigTOML: "model='grok-4.7'\nmodel_provider='grok2api'", ModelsJSON: &catalog})
	if err != nil {
		t.Fatal(err)
	}
	m := &Multiplexer{store: s}
	for _, method := range []string{"turn/start", "thread/settings/update", "thread/resume"} {
		key := "effort"
		if method == "thread/resume" {
			key = "reasoningEffort"
		}
		for _, effort := range []string{"none", "xhigh"} {
			p := map[string]any{key: effort, "collaborationMode": map[string]any{"settings": map[string]any{"reasoning_effort": effort}}}
			raw, _ := json.Marshal(p)
			msg := m.applyProvider(a.ID, protocol.Request(method, nil, raw))
			json.Unmarshal(msg.Params, &p)
			want := effort
			if effort == "none" {
				want = "medium"
			}
			if p[key] != want || p["collaborationMode"].(map[string]any)["settings"].(map[string]any)["reasoning_effort"] != want {
				t.Fatalf("%s: %s", method, msg.Params)
			}
		}
	}
}
