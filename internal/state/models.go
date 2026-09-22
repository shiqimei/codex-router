package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type CatalogModel struct {
	Slug             string `json:"slug"`
	DisplayName      string `json:"display_name"`
	Visibility       string `json:"visibility"`
	DefaultReasoning string `json:"default_reasoning_level"`
	Reasoning        []struct {
		Effort string `json:"effort"`
	} `json:"supported_reasoning_levels"`
}

func validateModelsJSON(value, defaultModel string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if len(value) > 4<<20 {
		return errors.New("models.json exceeds 4 MiB")
	}
	var catalog struct {
		Models []CatalogModel `json:"models"`
	}
	if err := json.Unmarshal([]byte(value), &catalog); err != nil {
		return errors.New("invalid models.json")
	}
	if len(catalog.Models) == 0 {
		return errors.New("models.json must contain a nonempty models array")
	}
	seen := map[string]bool{}
	found := false
	for _, m := range catalog.Models {
		if strings.TrimSpace(m.Slug) == "" || seen[m.Slug] {
			return errors.New("models.json slugs must be nonempty and unique")
		}
		seen[m.Slug] = true
		if m.Slug == defaultModel {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("configured model %q is missing from models.json", defaultModel)
	}
	return nil
}
func readModels(home string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(home, "models.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}
func writeModels(home string, data []byte) error {
	path := filepath.Join(home, "models.json")
	if len(strings.TrimSpace(string(data))) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return privateWrite(path, data)
}
func (s *Store) Models(account Account) []CatalogModel {
	data, err := readModels(account.CodexHome)
	if err != nil {
		return nil
	}
	var catalog struct {
		Models []CatalogModel `json:"models"`
	}
	if json.Unmarshal(data, &catalog) != nil {
		return nil
	}
	return catalog.Models
}
func (s *Store) ThreadModel(thread, account string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.threadModels[thread][account]
}
func (s *Store) SetThreadModel(thread, account, model string) error {
	if thread == "" || account == "" || model == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.threadModels == nil {
		s.threadModels = map[string]map[string]string{}
	}
	if s.threadModels[thread] == nil {
		s.threadModels[thread] = map[string]string{}
	}
	old := s.threadModels[thread][account]
	if old == model {
		return nil
	}
	s.threadModels[thread][account] = model
	if err := s.saveLocked(); err != nil {
		s.threadModels[thread][account] = old
		return err
	}
	return nil
}
