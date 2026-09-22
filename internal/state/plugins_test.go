package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSharedPluginCacheKeepsCUADefinitionsButNotCredentials(t *testing.T) {
	primary := t.TempDir()
	account := t.TempDir()
	cache := filepath.Join(primary, "plugins", "cache", "openai-bundled", "unified-computer-use", "1")
	if err := os.MkdirAll(cache, 0700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(cache, ".mcp.json"), []byte(`{"mcpServers":{"cua_repl":{"enabled":true}}}`), 0600)
	os.WriteFile(filepath.Join(primary, "auth.json"), []byte("private auth"), 0600)
	if err := sharePluginCache(primary, account); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(account, "plugins", "cache", "openai-bundled", "unified-computer-use", "1", ".mcp.json")
	if data, err := os.ReadFile(path); err != nil || len(data) == 0 {
		t.Fatalf("CUA definitions missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(account, "auth.json")); !os.IsNotExist(err) {
		t.Fatal("account auth was shared")
	}
	if _, err := os.Stat(filepath.Join(account, "plugins", ".plugin-appserver")); !os.IsNotExist(err) {
		t.Fatal("private plugin app-server directory was shared")
	}
	// Updates from the desktop must be visible without overwriting account config.
	os.WriteFile(filepath.Join(cache, ".mcp.json"), []byte("updated"), 0600)
	if data, _ := os.ReadFile(path); string(data) != "updated" {
		t.Fatal("plugin definition update not visible")
	}
}
func TestExistingAccountPluginCacheIsPreserved(t *testing.T) {
	primary := t.TempDir()
	account := t.TempDir()
	os.MkdirAll(filepath.Join(primary, "plugins", "cache", "new"), 0700)
	os.MkdirAll(filepath.Join(account, "plugins", "cache", "local"), 0700)
	own := filepath.Join(account, "plugins", "cache", "local", "keep")
	os.WriteFile(own, []byte("mine"), 0600)
	if err := sharePluginCache(primary, account); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(own); string(data) != "mine" {
		t.Fatal("existing plugin changed")
	}
	if _, err := os.Stat(filepath.Join(account, "plugins", "cache", "new")); err != nil {
		t.Fatal(err)
	}
}

func TestIsolatedProviderGetsNativeCUAServerWithoutPluginAuth(t *testing.T) {
	source := t.TempDir()
	dir := filepath.Join(source, "plugins", "unified-computer-use")
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(`{"mcpServers":{"cua_repl":{"command":"/native/node","args":["/native/cua-repl.mjs"],"enabled":true,"tools":{"js":{"output_token_limit":25000}},"env":{"CUA_REPL_ENABLED_SURFACES":"browser,computer"}}}}`), 0600)
	config := map[string]any{"plugins": map[string]any{"unified-computer-use@openai-bundled": map[string]any{"enabled": true}}, "marketplaces": map[string]any{"openai-bundled": map[string]any{"source_type": "local", "source": source}}}
	if err := addNativeCUAServer(t.TempDir(), config, config); err != nil {
		t.Fatal(err)
	}
	server := config["mcp_servers"].(map[string]any)["cua_repl"].(map[string]any)
	if server["command"] != "/native/node" {
		t.Fatal("native CUA runtime not propagated")
	}
	limit := server["tools"].(map[string]any)["js"].(map[string]any)["output_token_limit"]
	if _, ok := limit.(int64); !ok {
		t.Fatalf("integer MCP config must survive TOML conversion: %T", limit)
	}
	// Respect an explicitly configured or disabled server.
	config["mcp_servers"].(map[string]any)["cua_repl"] = map[string]any{"enabled": false}
	addNativeCUAServer(t.TempDir(), config, config)
	if config["mcp_servers"].(map[string]any)["cua_repl"].(map[string]any)["enabled"] != false {
		t.Fatal("overrode explicit CUA disable")
	}
}

func TestNativeCUAUsesActiveInstalledDefinitionNotDisabledTemplate(t *testing.T) {
	home := t.TempDir()
	source := filepath.Join(home, "marketplace")
	plugin := filepath.Join(source, "plugins", "unified-computer-use")
	os.MkdirAll(filepath.Join(plugin, ".codex-plugin"), 0700)
	os.WriteFile(filepath.Join(plugin, ".codex-plugin", "plugin.json"), []byte(`{"version":"1.2.3"}`), 0600)
	os.WriteFile(filepath.Join(plugin, ".mcp.json"), []byte(`{"mcpServers":{"cua_repl":{"command":"node","enabled":false}}}`), 0600)
	installed := filepath.Join(home, "plugins", "cache", "openai-bundled", "unified-computer-use", "1.2.3")
	os.MkdirAll(installed, 0700)
	os.WriteFile(filepath.Join(installed, ".mcp.json"), []byte(`{"mcpServers":{"cua_repl":{"command":"/active/node","enabled":true}}}`), 0600)
	config := map[string]any{"plugins": map[string]any{"unified-computer-use@openai-bundled": map[string]any{"enabled": true}}, "marketplaces": map[string]any{"openai-bundled": map[string]any{"source_type": "local", "source": source}}}
	if err := addNativeCUAServer(home, config, config); err != nil {
		t.Fatal(err)
	}
	if config["mcp_servers"].(map[string]any)["cua_repl"].(map[string]any)["command"] != "/active/node" {
		t.Fatal("did not resolve installed native runtime")
	}
}
