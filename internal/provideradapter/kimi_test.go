package provideradapter

import (
	"context"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestKimiPreciseWeeklyRatioOverridesRoundedLegacy(t *testing.T) {
	usage, err := parseKimiUsage([]byte(`{"usage":{"limit":"100","remaining":"100"},"usages":{"limit_7d":{"used_ratio":0.000853,"reset_time":"2026-09-23T17:41:50Z"},"limit_5h":{"used_ratio":0.9}}}`))
	if err != nil || usage.RemainingPercent == nil || math.Abs(*usage.RemainingPercent-99.9147) > 0.00001 || usage.ResetsAt == 0 {
		t.Fatalf("unexpected quota: %+v %v", usage, err)
	}
}
func TestKimiLegacyAndInvalidQuota(t *testing.T) {
	for _, raw := range []string{`{"usage":{"limit":"200","remaining":"50"}}`, `{"usage":{"limit":200,"remaining":50}}`} {
		u, err := parseKimiUsage([]byte(raw))
		if err != nil || *u.RemainingPercent != 25 {
			t.Fatal(u, err)
		}
	}
	for _, raw := range []string{`{}`, `{"usage":{"limit":0,"remaining":0}}`, `{"usages":{"limit_7d":{"used_ratio":null}}}`, `{"usages":{"limit_7d":{"used_ratio":"NaN"}}}`, `{"usage":{"limit":100}}`} {
		if _, err := parseKimiUsage([]byte(raw)); err == nil {
			t.Fatal("invalid data shown as balance:", raw)
		}
	}
}
func TestKimiAdapterOnlyMatchesOfficialCodingOrigin(t *testing.T) {
	a := Kimi{}
	for _, base := range []string{"https://api.kimi.ai/coding/v1", "https://api.kimi.com/coding/"} {
		if !a.Match(Config{BaseURL: base}) {
			t.Fatal(base)
		}
	}
	for _, base := range []string{"http://api.kimi.ai/coding/v1", "https://api.kimi.ai.evil/coding/v1", "https://api.kimi.ai:444/coding/v1", "https://api.kimi.ai/v1", "https://user:secret@api.kimi.com/coding/v1"} {
		if a.Match(Config{BaseURL: base}) {
			t.Fatal(base)
		}
	}
}
func TestKimiRequestAuthAndRedirectAreBounded(t *testing.T) {
	requests := 0
	a := Kimi{Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.String() != "https://api.kimi.ai/coding/v1/usages" || r.Header.Get("Authorization") != "Bearer test-secret" || r.Header.Get("User-Agent") == "" {
			t.Fatal("request contract mismatch")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"usages":{"limit_7d":{"used_ratio":0.25}}}`))}, nil
	})}}
	u := a.Read(context.Background(), Config{BaseURL: "https://api.kimi.ai/coding/v1?x=1", Credential: "test-secret"})
	if u.RemainingPercent == nil || *u.RemainingPercent != 75 || requests != 1 {
		t.Fatal(u)
	}
	a.Client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://other.example/leak"}}, Body: io.NopCloser(strings.NewReader("test-secret"))}, nil
	})
	u = a.Read(context.Background(), Config{BaseURL: "https://api.kimi.ai/coding/v1", Credential: "test-secret"})
	if requests != 2 || u.Error == "" || strings.Contains(u.Error, "test-secret") || u.RemainingPercent != nil {
		t.Fatal(u, requests)
	}
}
func TestRegistrySelectsGrokAndKimi(t *testing.T) {
	if _, ok := Resolve(Config{BaseURL: "https://api.kimi.ai/coding/v1"}).(Kimi); !ok {
		t.Fatal("Kimi missing")
	}
	if _, ok := Resolve(Config{BaseURL: "http://localhost:8000/v1", Binding: Binding{Type: "grok2api-sqlite", BaseURL: "http://127.0.0.1:8000/v1"}}).(Grok); !ok {
		t.Fatal("Grok missing")
	}
	if Resolve(Config{BaseURL: "https://other.example/v1"}) != nil {
		t.Fatal("unknown provider must retain API badge")
	}
}
