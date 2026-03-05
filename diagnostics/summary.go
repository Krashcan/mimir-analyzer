package diagnostics

import (
	"encoding/json"
	"math"
	"strconv"
	"time"
)

type QuerySummary struct {
	SeriesCount       int     `json:"series_count"`
	MaxValue          float64 `json:"max_value"`
	MaxTimestamp      string  `json:"max_timestamp,omitempty"`
	AvgValue          float64 `json:"avg_value"`
	NonZeroPercentage float64 `json:"non_zero_percentage"`
	Trend             string  `json:"trend"`
}

type matrixSeries struct {
	Metric map[string]string `json:"metric"`
	Values [][]json.RawMessage `json:"values"`
}

func ComputeSummary(raw json.RawMessage) (*QuerySummary, error) {
	var series []matrixSeries
	if err := json.Unmarshal(raw, &series); err != nil {
		return &QuerySummary{}, nil
	}

	if len(series) == 0 {
		return &QuerySummary{}, nil
	}

	var allValues []float64
	var allTimestamps []float64
	maxVal := math.Inf(-1)
	var maxTs float64
	nonZero := 0
	total := 0

	for _, s := range series {
		for _, point := range s.Values {
			if len(point) < 2 {
				continue
			}
			var ts float64
			if err := json.Unmarshal(point[0], &ts); err != nil {
				continue
			}
			var valStr string
			if err := json.Unmarshal(point[1], &valStr); err != nil {
				continue
			}
			val, err := strconv.ParseFloat(valStr, 64)
			if err != nil {
				continue
			}

			allValues = append(allValues, val)
			allTimestamps = append(allTimestamps, ts)
			total++
			if val != 0 {
				nonZero++
			}
			if val > maxVal {
				maxVal = val
				maxTs = ts
			}
		}
	}

	if total == 0 {
		return &QuerySummary{SeriesCount: len(series)}, nil
	}

	sum := 0.0
	for _, v := range allValues {
		sum += v
	}

	nzPct := 0.0
	if total > 0 {
		nzPct = float64(nonZero) / float64(total) * 100
	}

	trend := detectTrend(allValues)

	return &QuerySummary{
		SeriesCount:       len(series),
		MaxValue:          maxVal,
		MaxTimestamp:      time.Unix(int64(maxTs), 0).UTC().Format(time.RFC3339),
		AvgValue:          sum / float64(total),
		NonZeroPercentage: nzPct,
		Trend:             trend,
	}, nil
}

func detectTrend(values []float64) string {
	n := len(values)
	if n < 2 {
		return "stable"
	}

	// Linear regression: slope
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for i, v := range values {
		x := float64(i)
		sumX += x
		sumY += v
		sumXY += x * v
		sumX2 += x * x
	}
	nf := float64(n)
	slope := (nf*sumXY - sumX*sumY) / (nf*sumX2 - sumX*sumX)
	mean := sumY / nf

	// Coefficient of variation
	sumSqDiff := 0.0
	for _, v := range values {
		d := v - mean
		sumSqDiff += d * d
	}
	stddev := math.Sqrt(sumSqDiff / nf)
	cv := 0.0
	if mean != 0 {
		cv = stddev / math.Abs(mean)
	}

	// Spiky: high coefficient of variation and weak trend
	if cv > 0.5 {
		// Normalize slope relative to mean
		normalizedSlope := 0.0
		if mean != 0 {
			normalizedSlope = math.Abs(slope) / math.Abs(mean)
		}
		if normalizedSlope < 0.1 || cv > 0.8 {
			return "spiky"
		}
	}

	// Normalized slope threshold
	if mean != 0 {
		normalizedSlope := slope / math.Abs(mean)
		if normalizedSlope > 0.05 {
			return "increasing"
		}
		if normalizedSlope < -0.05 {
			return "decreasing"
		}
	} else {
		// mean is 0, check absolute slope
		if slope > 0.001 {
			return "increasing"
		}
		if slope < -0.001 {
			return "decreasing"
		}
	}

	return "stable"
}
