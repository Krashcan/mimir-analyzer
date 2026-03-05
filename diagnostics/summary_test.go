package diagnostics

import (
	"encoding/json"
	"math"
	"testing"
)

func TestComputeSummary_SingleSeriesIncreasing(t *testing.T) {
	raw := json.RawMessage(`[{"metric":{"__name__":"test"},"values":[[1000,"1"],[1060,"2"],[1120,"3"],[1180,"4"],[1240,"5"]]}]`)
	s, err := ComputeSummary(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.SeriesCount != 1 {
		t.Errorf("SeriesCount = %d, want 1", s.SeriesCount)
	}
	if s.MaxValue != 5 {
		t.Errorf("MaxValue = %f, want 5", s.MaxValue)
	}
	if s.AvgValue != 3 {
		t.Errorf("AvgValue = %f, want 3", s.AvgValue)
	}
	if s.Trend != "increasing" {
		t.Errorf("Trend = %q, want %q", s.Trend, "increasing")
	}
}

func TestComputeSummary_Decreasing(t *testing.T) {
	raw := json.RawMessage(`[{"metric":{"__name__":"test"},"values":[[1000,"10"],[1060,"8"],[1120,"6"],[1180,"4"],[1240,"2"]]}]`)
	s, err := ComputeSummary(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Trend != "decreasing" {
		t.Errorf("Trend = %q, want %q", s.Trend, "decreasing")
	}
}

func TestComputeSummary_Stable(t *testing.T) {
	raw := json.RawMessage(`[{"metric":{"__name__":"test"},"values":[[1000,"5"],[1060,"5"],[1120,"5"],[1180,"5"],[1240,"5"]]}]`)
	s, err := ComputeSummary(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Trend != "stable" {
		t.Errorf("Trend = %q, want %q", s.Trend, "stable")
	}
}

func TestComputeSummary_Spiky(t *testing.T) {
	raw := json.RawMessage(`[{"metric":{"__name__":"test"},"values":[[1000,"1"],[1060,"100"],[1120,"2"],[1180,"99"],[1240,"3"]]}]`)
	s, err := ComputeSummary(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Trend != "spiky" {
		t.Errorf("Trend = %q, want %q", s.Trend, "spiky")
	}
}

func TestComputeSummary_EmptyResult(t *testing.T) {
	raw := json.RawMessage(`[]`)
	s, err := ComputeSummary(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.SeriesCount != 0 {
		t.Errorf("SeriesCount = %d, want 0", s.SeriesCount)
	}
}

func TestComputeSummary_MultiSeries(t *testing.T) {
	raw := json.RawMessage(`[
		{"metric":{"instance":"a"},"values":[[1000,"2"],[1060,"4"]]},
		{"metric":{"instance":"b"},"values":[[1000,"6"],[1060,"8"]]}
	]`)
	s, err := ComputeSummary(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.SeriesCount != 2 {
		t.Errorf("SeriesCount = %d, want 2", s.SeriesCount)
	}
	if s.MaxValue != 8 {
		t.Errorf("MaxValue = %f, want 8", s.MaxValue)
	}
	expectedAvg := (2.0 + 4.0 + 6.0 + 8.0) / 4.0
	if math.Abs(s.AvgValue-expectedAvg) > 0.01 {
		t.Errorf("AvgValue = %f, want %f", s.AvgValue, expectedAvg)
	}
}

func TestComputeSummary_WithZeros(t *testing.T) {
	raw := json.RawMessage(`[{"metric":{"__name__":"test"},"values":[[1000,"0"],[1060,"5"],[1120,"0"],[1180,"3"],[1240,"0"]]}]`)
	s, err := ComputeSummary(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedPct := 40.0 // 2 out of 5 are non-zero
	if math.Abs(s.NonZeroPercentage-expectedPct) > 0.01 {
		t.Errorf("NonZeroPercentage = %f, want %f", s.NonZeroPercentage, expectedPct)
	}
}

func TestComputeSummary_MaxTimestamp(t *testing.T) {
	raw := json.RawMessage(`[{"metric":{"__name__":"test"},"values":[[1000,"1"],[1060,"5"],[1120,"3"]]}]`)
	s, err := ComputeSummary(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.MaxTimestamp != "1970-01-01T00:17:40Z" {
		t.Errorf("MaxTimestamp = %q, want %q", s.MaxTimestamp, "1970-01-01T00:17:40Z")
	}
}
