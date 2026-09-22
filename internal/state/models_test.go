package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModelsJSONIsIsolatedAndSurvivesEdits(t *testing.T) {
	primary := t.TempDir()
	os.WriteFile(filepath.Join(primary, "config.toml"), []byte("model_context_window=9999999\nmodel_catalog_json='/primary/catalog.json'\n"), 0600)
	s, err := Open(t.TempDir(), primary)
	if err != nil {
		t.Fatal(err)
	}
	catalog := `{"models":[{"slug":"one","display_name":"One"},{"slug":"two","display_name":"Two"}]}`
	a, err := s.AddProvider(ProviderInput{Label: "Catalog", ConfigTOML: "model='one'\nmodel_provider='custom'", ModelsJSON: &catalog})
	if err != nil {
		t.Fatal(err)
	}
	config, _ := os.ReadFile(filepath.Join(a.CodexHome, "config.toml"))
	if !strings.Contains(string(config), filepath.Join(a.CodexHome, "models.json")) || strings.Contains(string(config), "9999999") {
		t.Fatal("catalog isolation failed")
	}
	details, _ := s.ProviderDetails(a.ID)
	if details.ModelsJSON != catalog {
		t.Fatal("catalog not editable")
	}
	_, err = s.UpdateProvider(a.ID, ProviderUpdate{ProviderInput: ProviderInput{Label: a.Label, ConfigTOML: "model='two'\nmodel_provider='custom'"}, Revision: details.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Models(a)) != 2 {
		t.Fatal("omitted modelsJson erased catalog")
	}
	details, _ = s.ProviderDetails(a.ID)
	empty := ""
	_, err = s.UpdateProvider(a.ID, ProviderUpdate{ProviderInput: ProviderInput{Label: a.Label, ConfigTOML: "model='two'\nmodel_provider='custom'", ModelsJSON: &empty}, Revision: details.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(a.CodexHome, "models.json")); !os.IsNotExist(err) {
		t.Fatal("explicit clear failed")
	}
	s.SetThreadModel("thread", a.ID, "two")
	reopened, err := Open(s.Root(), primary)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.ThreadModel("thread", a.ID) != "two" {
		t.Fatal("model selection lost on restart")
	}
}
func TestModelsJSONRejectsDuplicatesAndMissingDefault(t *testing.T) {
	for _, value := range []string{`{}`, `{"models":[]}`, `{"models":[{"slug":"one"},{"slug":"one"}]}`, `{"models":[{"slug":"two"}]}`} {
		if validateModelsJSON(value, "one") == nil {
			t.Fatalf("accepted invalid catalog %s", value)
		}
	}
}
