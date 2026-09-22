package mux

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPortableHistoryStripsOpaqueStateButPreservesToolsAndMessages(t *testing.T) {
	source := []byte(`{"timestamp":"now","type":"session_meta","payload":{"id":"thread-1"}}
{"type":"response_item","payload":{"type":"reasoning","encrypted_content":"account-bound-secret","summary":[{"type":"summary_text","text":"internal"}]}}
{"type":"response_item","payload":{"type":"function_call","call_id":"call-1","name":"exec","arguments":"{\"cmd\":\"echo hello\"}"}}
{"type":"response_item","payload":{"type":"function_call_output","call_id":"call-1","output":"hello"}}
{"type":"event_msg","payload":{"type":"agent_reasoning","text":"visible reasoning summary"}}
{"type":"compacted","payload":{"replacement_history":[{"type":"reasoning","encrypted_content":"other-secret"},{"type":"message","role":"user","content":[{"type":"input_text","text":"Keep the literal encrypted_content words"}]}]}}
{"type":"compacted","payload":{"replacement_history":[{"type":"compaction","encrypted_content":"compaction-secret"}]}}
`)
	cleaned, err := portableHistory(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"account-bound-secret", "other-secret", "compaction-secret"} {
		if strings.Contains(string(cleaned), secret) {
			t.Fatal("opaque content crossed account boundary")
		}
	}
	for _, preserved := range []string{"call-1", "function_call_output", "hello", "visible reasoning summary", "Keep the literal encrypted_content words"} {
		if !strings.Contains(string(cleaned), preserved) {
			t.Fatalf("lost %q", preserved)
		}
	}
	for _, line := range strings.Split(strings.TrimSpace(string(cleaned)), "\n") {
		if !json.Valid([]byte(line)) {
			t.Fatal("invalid output JSONL")
		}
	}
	if !strings.Contains(string(source), "account-bound-secret") {
		t.Fatal("modified source")
	}
}
func TestPortableHistoryRejectsTruncatedRollout(t *testing.T) {
	if _, err := portableHistory([]byte(`{"type":`)); err == nil {
		t.Fatal("accepted partial history")
	}
}

func TestPortableHistoryNormalizesLongProviderItemIDs(t *testing.T) {
	long := "ws_" + strings.Repeat("a", 80)
	source := []byte(`{"type":"response_item","payload":{"type":"web_search_call","id":"` + long + `","call_id":"keep-call-id","status":"completed"}}`)
	first, err := portableHistory(source)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := portableHistory(source)
	if string(first) != string(second) {
		t.Fatal("ID normalization must be deterministic")
	}
	var record struct {
		Payload struct {
			ID     string `json:"id"`
			CallID string `json:"call_id"`
		} `json:"payload"`
	}
	if err = json.Unmarshal(first, &record); err != nil {
		t.Fatal(err)
	}
	if len(record.Payload.ID) > 64 || !strings.HasPrefix(record.Payload.ID, "ws_") {
		t.Fatal("invalid portable item ID")
	}
	if record.Payload.CallID != "keep-call-id" {
		t.Fatal("tool join key changed")
	}
}
