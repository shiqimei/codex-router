package provideradapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Kimi struct{ Client *http.Client }

func (Kimi) Match(c Config) bool {
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.User != nil || u.Scheme != "https" || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return (host == "api.kimi.ai" || host == "api.kimi.com") && (u.Path == "/coding" || strings.HasPrefix(u.Path, "/coding/"))
}
func (a Kimi) Read(parent context.Context, c Config) Usage {
	failure := Usage{Adapter: "kimi", Kind: "weekly_remaining", Error: "Weekly usage unavailable"}
	if !a.Match(c) {
		return failure
	}
	if strings.TrimSpace(c.Credential) == "" {
		failure.Error = "Configure the provider API key to read weekly usage"
		return failure
	}
	ctx, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	u, _ := url.Parse(c.BaseURL)
	u.Path = "/coding/v1/usages"
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return failure
	}
	req.Header.Set("Authorization", "Bearer "+c.Credential)
	req.Header.Set("User-Agent", "KimiCLI/1.6")
	req.Header.Set("Accept", "application/json")
	client := &http.Client{}
	if a.Client != nil {
		copy := *a.Client
		client = &copy
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return failure
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		failure.Error = fmt.Sprintf("Weekly usage unavailable (HTTP %d)", resp.StatusCode)
		return failure
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return failure
	}
	result, err := parseKimiUsage(data)
	if err != nil {
		return failure
	}
	return result
}
func parseKimiUsage(data []byte) (Usage, error) {
	var d struct {
		Usages map[string]struct {
			UsedRatio json.RawMessage `json:"used_ratio"`
			ResetTime string          `json:"reset_time"`
		} `json:"usages"`
		Usage struct {
			Limit     json.RawMessage `json:"limit"`
			Remaining json.RawMessage `json:"remaining"`
			ResetTime string          `json:"resetTime"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return Usage{}, err
	}
	var remaining float64
	var reset string
	if weekly, ok := d.Usages["limit_7d"]; ok {
		ratio, err := quotaNumber(weekly.UsedRatio)
		if err != nil || ratio < 0 {
			return Usage{}, fmt.Errorf("invalid weekly ratio")
		}
		remaining = 100 * (1 - ratio)
		reset = weekly.ResetTime
	} else {
		limit, err := quotaNumber(d.Usage.Limit)
		if err != nil || limit <= 0 {
			return Usage{}, fmt.Errorf("missing weekly quota")
		}
		left, err := quotaNumber(d.Usage.Remaining)
		if err != nil || left < 0 {
			return Usage{}, fmt.Errorf("invalid weekly remaining")
		}
		remaining = left / limit * 100
		reset = d.Usage.ResetTime
	}
	remaining = math.Max(0, math.Min(100, remaining))
	result := Usage{Adapter: "kimi", Kind: "weekly_remaining", RemainingPercent: &remaining, UpdatedAt: time.Now().Unix()}
	if parsed, err := time.Parse(time.RFC3339Nano, reset); err == nil {
		result.ResetsAt = parsed.Unix()
	}
	return result, nil
}
func quotaNumber(raw json.RawMessage) (float64, error) {
	var n float64
	if len(raw) == 0 || string(raw) == "null" {
		return 0, fmt.Errorf("missing number")
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		v, err := strconv.ParseFloat(s, 64)
		n = v
		if err != nil {
			return 0, err
		}
	} else if err := json.Unmarshal(raw, &n); err != nil {
		return 0, err
	}
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, fmt.Errorf("invalid number")
	}
	return n, nil
}
