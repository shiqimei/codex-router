package provideradapter

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Grok struct{}

func (Grok) Match(c Config) bool {
	return c.Binding.Type == "grok2api-sqlite" && sameLocalUsageEndpoint(c.Binding.BaseURL, c.BaseURL)
}
func (Grok) Read(ctx context.Context, c Config) Usage {
	if !filepath.IsAbs(c.Binding.DatabasePath) {
		return Usage{Adapter: "grok", Kind: "tokens", Error: "Token usage unavailable"}
	}
	usage := readGrokWeightedQuota(ctx, c.Binding)
	tokens := readGrokTokenTotal(ctx, c.Binding.DatabasePath)
	usage.TotalTokens = tokens.TotalTokens
	return usage
}
func sameLocalUsageEndpoint(left, right string) bool {
	normalize := func(value string) string {
		u, err := url.Parse(value)
		if err != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return ""
		}
		host := strings.ToLower(u.Hostname())
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return ""
		}
		port := u.Port()
		if port == "" {
			if u.Scheme == "https" {
				port = "443"
			} else {
				port = "80"
			}
		}
		return u.Scheme + "://loopback:" + port + strings.TrimRight(u.Path, "/") + "?" + u.RawQuery
	}
	a, b := normalize(left), normalize(right)
	return a != "" && a == b
}
func readGrokTokenTotal(parent context.Context, path string) Usage {
	failure := Usage{Adapter: "grok", Kind: "tokens", Error: "Token usage unavailable"}
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return failure
	}
	ctx, cancel := context.WithTimeout(parent, 2500*time.Millisecond)
	defer cancel()
	// total_tokens already includes the provider's reported token accounting;
	// cached input and reasoning are not added a second time.
	command := exec.CommandContext(ctx, "sqlite3", "-readonly", "-batch", "-noheader", "-cmd", ".timeout 1500", path, "SELECT COALESCE(SUM(total_tokens), 0) FROM request_audits;")
	output, err := command.Output()
	if err != nil {
		return failure
	}
	total, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	if err != nil || total < 0 {
		return failure
	}
	return Usage{Adapter: "grok", Kind: "tokens", TotalTokens: &total, UpdatedAt: time.Now().Unix()}
}

type grokAccountQuota struct {
	ID     string   `json:"id"`
	Used   *float64 `json:"used_percent"`
	Period string   `json:"usage_period_type"`
	Reset  string   `json:"usage_period_end"`
	Synced string   `json:"synced_at"`
}

func readGrokWeightedQuota(parent context.Context, b Binding) Usage {
	failure := Usage{Adapter: "grok", Kind: "weekly_remaining", Error: "Weekly account quota unavailable"}
	if info, err := os.Stat(b.DatabasePath); err != nil || !info.Mode().IsRegular() {
		return failure
	}
	ctx, cancel := context.WithTimeout(parent, 2500*time.Millisecond)
	defer cancel()
	query := `SELECT CAST(a.id AS TEXT) AS id,b.credit_usage_percent AS used_percent,b.usage_period_type,b.usage_period_end,b.synced_at FROM provider_accounts a LEFT JOIN account_billing_snapshots b ON b.account_id=a.id ORDER BY a.id;`
	output, err := exec.CommandContext(ctx, "sqlite3", "-readonly", "-batch", "-json", "-cmd", ".timeout 1500", b.DatabasePath, query).Output()
	if err != nil {
		return failure
	}
	var rows []grokAccountQuota
	if json.Unmarshal(output, &rows) != nil {
		return failure
	}
	value, err := weightedGrokQuota(rows, b.AccountWeights, time.Now())
	if err != nil {
		return failure
	}
	return value
}
func weightedGrokQuota(rows []grokAccountQuota, weights map[string]float64, now time.Time) (Usage, error) {
	if len(rows) == 0 {
		return Usage{}, fmt.Errorf("no accounts")
	}
	value := Usage{Adapter: "grok", Kind: "weekly_remaining", AccountCount: len(rows), Weighting: "equal", UpdatedAt: now.Unix()}
	if len(weights) > 0 {
		value.Weighting = "configured"
	}
	sum, capacity := 0.0, 0.0
	for _, row := range rows {
		if row.Period != "USAGE_PERIOD_TYPE_WEEKLY" || row.Used == nil || math.IsNaN(*row.Used) || math.IsInf(*row.Used, 0) || *row.Used < 0 {
			return Usage{}, fmt.Errorf("missing weekly quota")
		}
		weight := 1.0
		if w, ok := weights[row.ID]; ok {
			weight = w
		}
		if weight <= 0 || math.IsNaN(weight) || math.IsInf(weight, 0) {
			return Usage{}, fmt.Errorf("invalid account weight")
		}
		sum += weight * math.Max(0, 100-*row.Used)
		capacity += weight
		reset, err := time.Parse(time.RFC3339Nano, row.Reset)
		if err != nil || !reset.After(now) {
			return Usage{}, fmt.Errorf("weekly snapshot needs refresh")
		}
		if value.ResetsAt == 0 || reset.Unix() < value.ResetsAt {
			value.ResetsAt = reset.Unix()
		}
		synced, err := time.Parse(time.RFC3339Nano, strings.Replace(row.Synced, " ", "T", 1))
		if err == nil && (value.SyncedAt == 0 || synced.Unix() < value.SyncedAt) {
			value.SyncedAt = synced.Unix()
		}
	}
	remaining := sum / capacity
	value.RemainingPercent = &remaining
	return value, nil
}
