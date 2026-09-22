package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderEditPreservesSecretsAndDetectsStaleEditor(t *testing.T) {
	s, err := Open(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.AddProvider(ProviderInput{Label: "Old", ConfigTOML: "model='old'\nmodel_provider='custom'", Environment: map[string]string{"API_KEY": "private-test-secret"}})
	if err != nil {
		t.Fatal(err)
	}
	original, err := s.ProviderDetails(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(original)
	if strings.Contains(string(encoded), "private-test-secret") {
		t.Fatal("secret leaked in edit response")
	}
	if len(original.EnvironmentKeys) != 1 || original.EnvironmentKeys[0] != "API_KEY" {
		t.Fatal("missing saved environment key metadata")
	}
	input := ProviderUpdate{ProviderInput: ProviderInput{Label: "Edited", ConfigTOML: "model='new'\nmodel_provider='other'"}, Revision: original.Revision}
	updated, err := s.UpdateProvider(a.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != a.ID || updated.Model != "new" || updated.Provider != "other" || updated.Label != "Edited" {
		t.Fatal("edit did not update stable profile")
	}
	data, _ := os.ReadFile(filepath.Join(a.CodexHome, "provider-env.json"))
	if !strings.Contains(string(data), "private-test-secret") {
		t.Fatal("blank secret field erased saved credential")
	}
	if _, err = s.UpdateProvider(a.ID, input); err == nil {
		t.Fatal("stale editor overwrote newer configuration")
	}
	latest, _ := s.ProviderDetails(a.ID)
	input.Revision = latest.Revision
	input.Environment = map[string]string{}
	if _, err = s.UpdateProvider(a.ID, input); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(a.CodexHome, "provider-env.json"))
	if string(data) != "{}" {
		t.Fatal("explicit environment clear failed")
	}
	latest, _ = s.ProviderDetails(a.ID)
	input.Revision = latest.Revision
	input.ConfigTOML = "broken = '"
	if _, err = s.UpdateProvider(a.ID, input); err == nil {
		t.Fatal("accepted invalid edit")
	}
	unchanged, _ := s.ProviderDetails(a.ID)
	if unchanged.Revision != latest.Revision {
		t.Fatal("invalid edit mutated profile")
	}
}
func TestProviderDeleteRetainsHistoryAndCanBeUndone(t *testing.T) {
	s, err := Open(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.AddProvider(ProviderInput{Label: "Keep history", ConfigTOML: "model='m'\nmodel_provider='p'"})
	if err != nil {
		t.Fatal(err)
	}
	s.SetThreadOwner("thread", a.ID)
	s.SetDefaultAccount(a.ID)
	history := filepath.Join(a.CodexHome, "history.txt")
	os.WriteFile(history, []byte("keep"), 0600)
	if _, err = s.DeleteProvider(a.ID); err != nil {
		t.Fatal(err)
	}
	deleted, _ := s.Account(a.ID)
	if deleted.Enabled || deleted.DeletedAt == 0 || s.DefaultAccount() != "" {
		t.Fatal("deleted provider remains selectable")
	}
	if owner, _ := s.ThreadOwner("thread"); owner != a.ID {
		t.Fatal("thread ownership lost")
	}
	if data, _ := os.ReadFile(history); string(data) != "keep" {
		t.Fatal("history deleted")
	}
	reopened, err := Open(s.Root(), s.primaryCodexHome)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := reopened.RestoreProvider(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.DeletedAt != 0 || !restored.Enabled {
		t.Fatal("undo failed")
	}
	if _, err = s.DeleteProvider("primary"); err == nil {
		t.Fatal("deleted primary subscription")
	}
}
