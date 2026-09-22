package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// ProviderInput accepts native Codex configuration rather than translating a
// subset of provider protocols. Secrets are kept in private files, never state.json.
type ProviderInput struct {
	ModelsJSON  *string           `json:"modelsJson,omitempty"`
	Label       string            `json:"label"`
	ConfigTOML  string            `json:"configToml"`
	Environment map[string]string `json:"environment,omitempty"`
}

func (s *Store) AddProvider(input ProviderInput) (Account, error) {
	provider, model, err := validateProvider(input)
	if err != nil {
		return Account{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := randomID()
	if err != nil {
		return Account{}, err
	}
	account := Account{ID: id, Label: strings.TrimSpace(input.Label), Kind: "provider", Provider: provider, Model: model, CodexHome: filepath.Join(s.root, "accounts", id, "codex-home"), Enabled: true, CreatedAt: time.Now().Unix()}
	if account.Label == "" {
		account.Label = provider
	}
	if err = os.MkdirAll(account.CodexHome, 0700); err != nil {
		return Account{}, err
	}
	saved := false
	defer func() {
		if !saved {
			_ = os.RemoveAll(filepath.Dir(account.CodexHome))
		}
	}()
	if err = os.WriteFile(filepath.Join(account.CodexHome, "provider.toml"), []byte(input.ConfigTOML), 0600); err != nil {
		return Account{}, err
	}
	if input.ModelsJSON != nil {
		if err = writeModels(account.CodexHome, []byte(*input.ModelsJSON)); err != nil {
			return Account{}, err
		}
	}
	data, _ := json.Marshal(input.Environment)
	if err = os.WriteFile(filepath.Join(account.CodexHome, "provider-env.json"), data, 0600); err != nil {
		return Account{}, err
	}
	if err = s.syncAccountConfig(account); err != nil {
		return Account{}, err
	}
	s.accounts = append(s.accounts, account)
	if err = s.saveLocked(); err != nil {
		s.accounts = s.accounts[:len(s.accounts)-1]
		return Account{}, err
	}
	saved = true
	return account, nil
}

func (s *Store) syncAccountConfig(account Account) error {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	if err := sharePluginCache(s.primaryCodexHome, account.CodexHome); err != nil {
		return err
	}
	if account.Kind != "provider" {
		return syncIsolatedConfig(s.primaryCodexHome, account.CodexHome)
	}
	// Publish a complete config in one rename: readers never see the primary
	// provider between copying the shared settings and applying this overlay.
	base, err := readConfig(filepath.Join(s.primaryCodexHome, "config.toml"))
	if err != nil {
		return err
	}
	config := map[string]any{}
	if err = toml.Unmarshal(base, &config); err != nil {
		return err
	}
	delete(config, "projects")
	delete(config, "model_catalog_json")
	delete(config, "model_context_window")
	delete(config, "model_auto_compact_token_limit")
	delete(config, "model_reasoning_effort")
	delete(config, "model_reasoning_summary")
	delete(config, "service_tier")
	delete(config, "forced_login_method")
	delete(config, "forced_chatgpt_workspace_id")
	path := filepath.Join(account.CodexHome, "config.toml")
	if old, readErr := readConfig(path); readErr == nil {
		var previous map[string]any
		if toml.Unmarshal(old, &previous) == nil {
			if projects, ok := previous["projects"]; ok {
				config["projects"] = projects
			}
		}
	}
	data, err := os.ReadFile(filepath.Join(account.CodexHome, "provider.toml"))
	if err != nil {
		return err
	}
	var overlay map[string]any
	if err = toml.Unmarshal(data, &overlay); err != nil {
		return err
	}
	mergeConfig(config, overlay)
	if err = addNativeCUAServer(s.primaryCodexHome, config, config); err != nil {
		return err
	}
	if catalog, err := readModels(account.CodexHome); err != nil {
		return err
	} else if len(catalog) > 0 {
		config["model_catalog_json"] = filepath.Join(account.CodexHome, "models.json")
	}
	config["cli_auth_credentials_store"] = "file"
	config["mcp_oauth_credentials_store"] = "file"
	data, err = toml.Marshal(config)
	if err != nil {
		return err
	}
	if err = os.WriteFile(path+".provider.tmp", data, 0600); err != nil {
		return err
	}
	return os.Rename(path+".provider.tmp", path)
}

func mergeConfig(base, overlay map[string]any) {
	for k, v := range overlay {
		if m, ok := v.(map[string]any); ok {
			if b, ok := base[k].(map[string]any); ok {
				mergeConfig(b, m)
				continue
			}
		}
		base[k] = v
	}
}

func (s *Store) ChildConfig(account Account, args, env []string) ([]string, []string, error) {
	args = append([]string(nil), args...)
	env = append([]string(nil), env...)
	if account.Kind != "provider" {
		if !account.Controller {
			args = append(args, "-c", `model_provider="openai"`)
		}
		return args, env, nil
	}
	args = append(args, "-c", "model_provider="+strconv.Quote(account.Provider), "-c", "model="+strconv.Quote(account.Model))
	data, err := os.ReadFile(filepath.Join(account.CodexHome, "provider-env.json"))
	if err != nil {
		return nil, nil, err
	}
	var secrets map[string]string
	if err = json.Unmarshal(data, &secrets); err != nil {
		return nil, nil, err
	}
	for k, v := range secrets {
		prefix := k + "="
		var filtered []string
		for _, e := range env {
			if !strings.HasPrefix(e, prefix) {
				filtered = append(filtered, e)
			}
		}
		env = append(filtered, prefix+v)
	}
	return args, env, nil
}

func (s *Store) DefaultModel(account Account) string {
	data, err := os.ReadFile(filepath.Join(account.CodexHome, "config.toml"))
	if err != nil {
		return ""
	}
	var c map[string]any
	if toml.Unmarshal(data, &c) != nil {
		return ""
	}
	model, _ := c["model"].(string)
	return model
}

func (s *Store) DefaultReasoning(account Account) string {
	data, err := os.ReadFile(filepath.Join(account.CodexHome, "config.toml"))
	if err != nil {
		return "medium"
	}
	var c map[string]any
	if toml.Unmarshal(data, &c) != nil {
		return "medium"
	}
	effort, _ := c["model_reasoning_effort"].(string)
	if effort == "" {
		return "medium"
	}
	return effort
}
