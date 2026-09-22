package provideradapter

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalMetricsEndpointIsBoundToInstance(t *testing.T) {
	for _, test := range []struct {
		a, b string
		want bool
	}{{"http://localhost:8000/v1", "http://127.0.0.1:8000/v1/", true}, {"http://localhost:8000/v1", "http://localhost:8001/v1", false}, {"http://localhost:8000/v1", "https://example.com/v1", false}, {"http://localhost:8000/v1", "http://user:secret@localhost:8000/v1", false}} {
		if sameLocalUsageEndpoint(test.a, test.b) != test.want {
			t.Fatal("metrics endpoint isolation failed")
		}
	}
}
func TestGrokTotalUsesReportedTotalsWithoutDoubleCounting(t *testing.T) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 is required for native local metrics")
	}
	db := filepath.Join(t.TempDir(), "usage.db")
	err := exec.Command("sqlite3", db, "CREATE TABLE request_audits(total_tokens INTEGER,input_tokens INTEGER,cached_input_tokens INTEGER); INSERT INTO request_audits VALUES(269000000,250000000,240000000),(528111,500000,480000);").Run()
	if err != nil {
		t.Fatal(err)
	}
	result := readGrokTokenTotal(context.Background(), db)
	if result.TotalTokens == nil || *result.TotalTokens != 269528111 {
		t.Fatalf("wrong cumulative accounting: %+v", result)
	}
	missing := readGrokTokenTotal(context.Background(), filepath.Join(t.TempDir(), "missing.db"))
	if missing.TotalTokens != nil || missing.Error == "" {
		t.Fatal("missing source must not be presented as zero")
	}
}

func TestGrokWeightedWeeklyQuota(t *testing.T) {
	now := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	first, second := 4.0, 0.0
	rows := []grokAccountQuota{{ID: "1", Used: &first, Period: "USAGE_PERIOD_TYPE_WEEKLY", Reset: "2026-09-25T00:00:00Z"}, {ID: "2", Used: &second, Period: "USAGE_PERIOD_TYPE_WEEKLY", Reset: "2026-09-23T00:00:00Z"}}
	equal, err := weightedGrokQuota(rows, nil, now)
	if err != nil || *equal.RemainingPercent != 98 || equal.ResetsAt != time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC).Unix() {
		t.Fatal(equal, err)
	}
	weighted, err := weightedGrokQuota(rows, map[string]float64{"1": 3, "2": 1}, now)
	if err != nil || *weighted.RemainingPercent != 97 || weighted.AccountCount != 2 {
		t.Fatal(weighted, err)
	}
	rows[0].Used = nil
	if _, err := weightedGrokQuota(rows, nil, now); err == nil {
		t.Fatal("missing account quota must not be omitted")
	}
	rows[0].Used = &first
	rows[0].Reset = "2026-09-21T00:00:00Z"
	if _, err := weightedGrokQuota(rows, nil, now); err == nil {
		t.Fatal("expired snapshot presented as current")
	}
}
