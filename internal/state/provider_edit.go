package state

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type ProviderDetails struct {
	ModelsJSON      string   `json:"modelsJson"`
	ID              string   `json:"id"`
	Label           string   `json:"label"`
	ConfigTOML      string   `json:"configToml"`
	EnvironmentKeys []string `json:"environmentKeys"`
	Revision        string   `json:"revision"`
}
type ProviderUpdate struct {
	ProviderInput
	Revision string `json:"revision"`
}

func providerRevision(config, environment []byte, label string, catalog ...[]byte) string {
	if len(catalog) > 0 {
		environment = append(append(append([]byte{}, environment...), 0), catalog[0]...)
	}
	config = append(append([]byte(label), 0), config...)
	return fmt.Sprintf("%x", sha256.Sum256(append(append(append([]byte{}, config...), 0), environment...)))
}
func (s *Store) ProviderDetails(id string) (ProviderDetails, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, a := range s.accounts {
		if a.ID == id && a.Kind == "provider" && a.DeletedAt == 0 {
			config, err := os.ReadFile(filepath.Join(a.CodexHome, "provider.toml"))
			if err != nil {
				return ProviderDetails{}, err
			}
			env, err := os.ReadFile(filepath.Join(a.CodexHome, "provider-env.json"))
			if err != nil {
				return ProviderDetails{}, err
			}
			var values map[string]string
			if err = json.Unmarshal(env, &values); err != nil {
				return ProviderDetails{}, err
			}
			keys := make([]string, 0, len(values))
			for k := range values {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			catalog, err := readModels(a.CodexHome)
			if err != nil {
				return ProviderDetails{}, err
			}
			return ProviderDetails{ModelsJSON: string(catalog), ID: id, Label: a.Label, ConfigTOML: string(config), EnvironmentKeys: keys, Revision: providerRevision(config, env, a.Label, catalog)}, nil
		}
	}
	return ProviderDetails{}, errors.New("provider not found")
}
func validateProvider(input ProviderInput) (string, string, error) {
	var config map[string]any
	if toml.Unmarshal([]byte(input.ConfigTOML), &config) != nil {
		return "", "", errors.New("invalid Codex TOML; check syntax and duplicate keys")
	}
	provider, _ := config["model_provider"].(string)
	model, _ := config["model"].(string)
	if strings.TrimSpace(provider) == "" || strings.TrimSpace(model) == "" {
		return "", "", errors.New("model_provider and model are required")
	}
	if input.ModelsJSON != nil {
		if err := validateModelsJSON(*input.ModelsJSON, model); err != nil {
			return "", "", err
		}
	}
	for k, v := range input.Environment {
		if k == "" || strings.ContainsAny(k, "=\x00") || strings.ContainsRune(v, 0) || k == "CODEX_HOME" || k == "CODEX_SQLITE_HOME" {
			return "", "", errors.New("invalid environment variable")
		}
	}
	return provider, model, nil
}
func privateWrite(path string, data []byte) error {
	if err := os.WriteFile(path+".edit", data, 0600); err != nil {
		return err
	}
	return os.Rename(path+".edit", path)
}

// UpdateProvider publishes a complete native profile and restores its previous
// files/metadata on failure. A nil environment preserves saved secret values.
func (s *Store) UpdateProvider(id string, input ProviderUpdate) (Account, error) {
	provider, model, err := validateProvider(input.ProviderInput)
	if err != nil {
		return Account{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, a := range s.accounts {
		if a.ID != id {
			continue
		}
		if a.Kind != "provider" || a.DeletedAt != 0 {
			return Account{}, errors.New("provider not found")
		}
		cp := filepath.Join(a.CodexHome, "provider.toml")
		ep := filepath.Join(a.CodexHome, "provider-env.json")
		oldConfig, err := os.ReadFile(cp)
		if err != nil {
			return Account{}, err
		}
		oldEnv, err := os.ReadFile(ep)
		if err != nil {
			return Account{}, err
		}
		oldModels, err := readModels(a.CodexHome)
		if err != nil {
			return Account{}, err
		}
		if input.Revision == "" || input.Revision != providerRevision(oldConfig, oldEnv, a.Label, oldModels) {
			return Account{}, errors.New("provider changed; reopen the editor before saving")
		}
		models := oldModels
		if input.ModelsJSON != nil {
			models = []byte(*input.ModelsJSON)
		}
		if err = validateModelsJSON(string(models), model); err != nil {
			return Account{}, err
		}
		environment := oldEnv
		if input.Environment != nil {
			environment, _ = json.Marshal(input.Environment)
		}
		updated := a
		updated.Provider = provider
		updated.Model = model
		updated.Label = strings.TrimSpace(input.Label)
		if updated.Label == "" {
			updated.Label = provider
		}
		restore := func(cause error) (Account, error) {
			s.accounts[i] = a
			e1 := privateWrite(cp, oldConfig)
			e2 := privateWrite(ep, oldEnv)
			eCatalog := writeModels(a.CodexHome, oldModels)
			e3 := s.syncAccountConfig(a)
			return Account{}, errors.Join(cause, e1, e2, eCatalog, e3)
		}
		if err = privateWrite(cp, []byte(input.ConfigTOML)); err != nil {
			return Account{}, err
		}
		if err = privateWrite(ep, environment); err != nil {
			return restore(err)
		}
		if err = writeModels(a.CodexHome, models); err != nil {
			return restore(err)
		}
		if err = s.syncAccountConfig(updated); err != nil {
			return restore(err)
		}
		s.accounts[i] = updated
		if err = s.saveLocked(); err != nil {
			return restore(err)
		}
		return updated, nil
	}
	return Account{}, errors.New("provider not found")
}
func (s *Store) DeleteProvider(id string) (Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, a := range s.accounts {
		if a.ID != id {
			continue
		}
		if a.Kind != "provider" || a.Controller {
			return Account{}, errors.New("only providers can be deleted")
		}
		if a.DeletedAt != 0 {
			return a, nil
		}
		oldDefault := s.defaultAccount
		s.accounts[i].DeletedAt = time.Now().Unix()
		s.accounts[i].WasEnabled = a.Enabled
		s.accounts[i].Enabled = false
		if s.defaultAccount == id {
			s.defaultAccount = ""
		}
		if err := s.saveLocked(); err != nil {
			s.accounts[i] = a
			s.defaultAccount = oldDefault
			return Account{}, err
		}
		return s.accounts[i], nil
	}
	return Account{}, errors.New("provider not found")
}
func (s *Store) RestoreProvider(id string) (Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, a := range s.accounts {
		if a.ID != id {
			continue
		}
		if a.Kind != "provider" {
			return Account{}, errors.New("provider not found")
		}
		if a.DeletedAt == 0 {
			return a, nil
		}
		s.accounts[i].DeletedAt = 0
		s.accounts[i].Enabled = a.WasEnabled
		s.accounts[i].WasEnabled = false
		if err := s.saveLocked(); err != nil {
			s.accounts[i] = a
			return Account{}, err
		}
		return s.accounts[i], nil
	}
	return Account{}, errors.New("provider not found")
}
func (s *Store) OwnedThreads(id string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var ids []string
	for thread, owner := range s.owners {
		if owner == id {
			ids = append(ids, thread)
		}
	}
	sort.Strings(ids)
	return ids
}
