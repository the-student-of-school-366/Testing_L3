package loadtest

import (
	"math"
	"sort"
	"time"
)

// Metric is a latency and throughput summary for one set of samples.
type Metric struct {
	Samples    int     `json:"samples"`
	Errors     int     `json:"errors"`
	ErrorRate  float64 `json:"error_rate_percent"`
	Throughput float64 `json:"throughput_rps"`
	AverageMs  float64 `json:"average_ms"`
	MedianMs   float64 `json:"median_ms"`
	P95Ms      float64 `json:"p95_ms"`
	MinMs      float64 `json:"min_ms"`
	MaxMs      float64 `json:"max_ms"`
}

// LabelMetric is Metric for one request name.
type LabelMetric struct {
	Label string `json:"label"`
	Metric
}

func ms(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

// Percentile returns the nearest-rank percentile in [0,1].
// It does not modify the input slice.
func Percentile(values []time.Duration, p float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	rank := int(math.Ceil(p*float64(len(sorted)))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}

func summarize(samples []Sample, window time.Duration) Metric {
	if len(samples) == 0 {
		return Metric{}
	}
	lat := make([]time.Duration, len(samples))
	var sum int64
	var minD, maxD time.Duration
	errors := 0
	for i, s := range samples {
		lat[i] = s.Elapsed
		sum += int64(s.Elapsed)
		if i == 0 || s.Elapsed < minD {
			minD = s.Elapsed
		}
		if s.Elapsed > maxD {
			maxD = s.Elapsed
		}
		if !s.Success {
			errors++
		}
	}

	throughputWindow := window
	if throughputWindow <= 0 {
		start := samples[0].Start
		end := samples[0].Start.Add(samples[0].Elapsed)
		for _, s := range samples[1:] {
			if s.Start.Before(start) {
				start = s.Start
			}
			if stop := s.Start.Add(s.Elapsed); stop.After(end) {
				end = stop
			}
		}
		throughputWindow = end.Sub(start)
	}

	var rps float64
	if throughputWindow > 0 {
		rps = float64(len(samples)) / throughputWindow.Seconds()
	}

	return Metric{
		Samples:    len(samples),
		Errors:     errors,
		ErrorRate:  100 * float64(errors) / float64(len(samples)),
		Throughput: rps,
		AverageMs:  ms(time.Duration(sum / int64(len(samples)))),
		MedianMs:   ms(Percentile(lat, 0.50)),
		P95Ms:      ms(Percentile(lat, 0.95)),
		MinMs:      ms(minD),
		MaxMs:      ms(maxD),
	}
}

func summarizeByLabel(samples []Sample, window time.Duration) []LabelMetric {
	grouped := map[string][]Sample{}
	for _, s := range samples {
		grouped[s.Label] = append(grouped[s.Label], s)
	}
	out := make([]LabelMetric, 0, len(labelOrder))
	seen := map[string]bool{}
	for _, label := range labelOrder {
		group, ok := grouped[label]
		if !ok {
			continue
		}
		seen[label] = true
		out = append(out, LabelMetric{Label: label, Metric: summarize(group, window)})
	}
	var extra []string
	for label := range grouped {
		if !seen[label] {
			extra = append(extra, label)
		}
	}
	sort.Strings(extra)
	for _, label := range extra {
		out = append(out, LabelMetric{Label: label, Metric: summarize(grouped[label], window)})
	}
	return out
}
