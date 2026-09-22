package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestProviderPersistsNativeConfigWithoutLeakingCredentials(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "primary")
	if err := os.MkdirAll(primary, 0700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(primary, "config.toml"), []byte("model = 'subscription-model'\n[projects.'/private-project']\ntrust_level='trusted'\n[mcp_servers.example]\ncommand='example'\n"), 0600)
	s, err := Open(filepath.Join(root, "router"), primary)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.AddProvider(ProviderInput{Label: "Custom", ConfigTOML: "model='custom-model'\nmodel_provider='custom'\n[model_providers.custom]\nname='Custom'\nwire_api='responses'\nbase_url='http://localhost:9000/v1'\nenv_key='CUSTOM_TOKEN'\n[model_providers.custom.query_params]\napi-version='2026-01-01'\n", Environment: map[string]string{"CUSTOM_TOKEN": "test-secret-canary"}})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SyncManagedConfig(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(a.CodexHome, "config.toml"))
	var config map[string]any
	if err = toml.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if config["model_provider"] != "custom" || config["model"] != "custom-model" {
		t.Fatal("provider overlay overwritten")
	}
	if _, exists := config["projects"]; exists {
		t.Fatal("primary project trust leaked")
	}
	if config["mcp_servers"] == nil {
		t.Fatal("shared tool definitions missing")
	}
	provider := config["model_providers"].(map[string]any)["custom"].(map[string]any)
	if provider["query_params"].(map[string]any)["api-version"] != "2026-01-01" {
		t.Fatal("native provider fields were discarded")
	}
	state, _ := os.ReadFile(filepath.Join(s.Root(), "state.json"))
	if strings.Contains(string(state), "test-secret-canary") {
		t.Fatal("credential leaked into routing state")
	}
	_, env, err := s.ChildConfig(a, nil, []string{"CUSTOM_TOKEN=old"})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, v := range env {
		if strings.HasPrefix(v, "CUSTOM_TOKEN=") {
			count++
			if v != "CUSTOM_TOKEN=test-secret-canary" {
				t.Fatal("wrong credential")
			}
		}
	}
	if count != 1 {
		t.Fatal("duplicate credential variables")
	}
	for _, name := range []string{"config.toml", "provider.toml", "provider-env.json"} {
		info, _ := os.Stat(filepath.Join(a.CodexHome, name))
		if info.Mode().Perm() != 0600 {
			t.Fatal("provider file permissions")
		}
	}
	reopened, err := Open(s.Root(), primary)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := reopened.Account(a.ID); !ok || got.Provider != "custom" {
		t.Fatal("provider metadata lost on restart")
	}
}
func TestMalformedProviderDoesNotCreateAnAccount(t *testing.T) {
	s, err := Open(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.AddProvider(ProviderInput{ConfigTOML: "secret = 'broken"})
	if err == nil {
		t.Fatal("accepted invalid TOML")
	}
	if strings.Contains(err.Error(), "broken") {
		t.Fatal("parser exposed input")
	}
	if len(s.Accounts()) != 1 {
		t.Fatal("partial account persisted")
	}
}
