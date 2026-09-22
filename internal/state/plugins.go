package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Plugin enablement alone is insufficient: Codex resolves installed plugins
// from CODEX_HOME/plugins/cache. Share definitions, never account auth or the
// private plugin app-server directory. Existing account installs are preserved.
func sharePluginCache(primaryHome, accountHome string) error {
	if samePath(primaryHome, accountHome) {
		return nil
	}
	source := filepath.Join(primaryHome, "plugins", "cache")
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return linkMissingPluginEntries(source, filepath.Join(accountHome, "plugins", "cache"))
}
func linkMissingPluginEntries(source, destination string) error {
	info, err := os.Lstat(destination)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
			return err
		}
		absolute, err := filepath.Abs(source)
		if err != nil {
			return err
		}
		return os.Symlink(absolute, destination)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return fmt.Errorf("read shared plugin definitions: %w", err)
	}
	for _, entry := range entries {
		if err = linkMissingPluginEntries(filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// Local CUA does not require a ChatGPT subscription, but plugin installation
// discovery is account-dependent. Materialize the desktop's already enabled,
// trusted CUA MCP definition for isolated connections without copying app auth.
func addNativeCUAServer(primaryHome string, primary, merged map[string]any) error {
	plugins, _ := primary["plugins"].(map[string]any)
	plugin, _ := plugins["unified-computer-use@openai-bundled"].(map[string]any)
	if enabled, _ := plugin["enabled"].(bool); !enabled {
		return nil
	}
	servers, _ := merged["mcp_servers"].(map[string]any)
	if _, explicit := servers["cua_repl"]; explicit {
		return nil
	}
	marketplaces, _ := primary["marketplaces"].(map[string]any)
	marketplace, _ := marketplaces["openai-bundled"].(map[string]any)
	if marketplace["source_type"] != "local" {
		return nil
	}
	source, _ := marketplace["source"].(string)
	if !filepath.IsAbs(source) {
		return nil
	}
	path := filepath.Join(source, "plugins", "unified-computer-use", ".mcp.json")
	// Marketplace sources contain disabled placeholders. The desktop writes its
	// active absolute executable/env configuration into the versioned install cache.
	metadataPath := filepath.Join(source, "plugins", "unified-computer-use", ".codex-plugin", "plugin.json")
	if data, err := os.ReadFile(metadataPath); err == nil {
		var metadata struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(data, &metadata) == nil && metadata.Version != "" && filepath.Base(metadata.Version) == metadata.Version {
			installed := filepath.Join(primaryHome, "plugins", "cache", "openai-bundled", "unified-computer-use", metadata.Version, ".mcp.json")
			if _, err := os.Stat(installed); err == nil {
				path = installed
			}
		}
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var manifest map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err = decoder.Decode(&manifest); err != nil {
		return fmt.Errorf("read native CUA definition: %w", err)
	}
	mcp, _ := manifest["mcpServers"].(map[string]any)
	cua, _ := mcp["cua_repl"].(map[string]any)
	if cua == nil {
		return nil
	}
	if enabled, exists := cua["enabled"].(bool); exists && !enabled {
		return nil
	}
	if command, _ := cua["command"].(string); !filepath.IsAbs(command) {
		return fmt.Errorf("native CUA command must be absolute")
	}
	if servers == nil {
		servers = map[string]any{}
		merged["mcp_servers"] = servers
	}
	servers["cua_repl"] = nativeJSONValue(cua)
	return nil
}
func nativeJSONValue(value any) any {
	switch v := value.(type) {
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return n
		}
		n, _ := v.Float64()
		return n
	case map[string]any:
		for k, x := range v {
			v[k] = nativeJSONValue(x)
		}
		return v
	case []any:
		for i, x := range v {
			v[i] = nativeJSONValue(x)
		}
		return v
	default:
		return value
	}
}
