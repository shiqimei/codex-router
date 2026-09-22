package mux

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

// portableHistory removes provider/account-bound opaque inference state from a
// copied rollout. Plain messages, tool call IDs/results and visible event logs
// remain intact. The source history is never changed.
func portableHistory(data []byte) ([]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)
	var output bytes.Buffer
	line := 0
	for scanner.Scan() {
		line++
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		decoder.UseNumber()
		var record map[string]any
		if err := decoder.Decode(&record); err != nil {
			return nil, fmt.Errorf("invalid rollout JSON at line %d", line)
		}
		// An encrypted compaction replaces prior context with an account-bound blob.
		// Discard that compaction record so Codex replays the earlier plain items.
		if record["type"] == "compacted" && containsEncryptedCompaction(record) {
			continue
		}
		if record["type"] == "response_item" {
			if item, ok := record["payload"].(map[string]any); ok && opaqueItem(item) {
				continue
			}
		}
		clean := portableValue(record)
		encoded, err := json.Marshal(clean)
		if err != nil {
			return nil, err
		}
		output.Write(encoded)
		output.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read rollout: %w", err)
	}
	return output.Bytes(), nil
}
func opaqueItem(item map[string]any) bool {
	return item["type"] == "reasoning" || item["type"] == "item_reference" || item["type"] == "compaction"
}
func portableValue(value any) any {
	switch v := value.(type) {
	case []any:
		result := make([]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok && opaqueItem(m) {
				continue
			}
			result = append(result, portableValue(item))
		}
		return result
	case map[string]any:
		kind, _ := v["type"].(string)
		if kind == "message" || strings.HasSuffix(kind, "_call") || strings.HasSuffix(kind, "_call_output") {
			normalizeModelItemID(v)
		}
		for key, child := range v {
			switch key {
			case "encrypted_content", "encryptedContent", "previous_response_id", "previousResponseId":
				delete(v, key)
			default:
				v[key] = portableValue(child)
			}
		}
		return v
	default:
		return value
	}
}

func containsEncryptedCompaction(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		if v["type"] == "compaction" && v["encrypted_content"] != nil {
			return true
		}
		for _, child := range v {
			if containsEncryptedCompaction(child) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if containsEncryptedCompaction(child) {
				return true
			}
		}
	}
	return false
}

// Responses input item IDs have a 64-character limit. Some compatible providers
// generate longer IDs. Preserve a recognizable prefix and deterministic identity;
// call_id (the tool call/result join key) and the thread's own ID are untouched.
func normalizeModelItemID(item map[string]any) {
	id, ok := item["id"].(string)
	if !ok || len(id) <= 64 {
		return
	}
	prefix := "item_"
	if i := strings.IndexByte(id, '_'); i > 0 && i < 12 {
		prefix = id[:i+1]
	}
	sum := sha256.Sum256([]byte(id))
	item["id"] = fmt.Sprintf("%s%x", prefix, sum[:20])
}
