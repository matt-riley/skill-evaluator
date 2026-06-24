package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"
)

// computeBenchmark aggregates run results into a benchmark.json.
func computeBenchmark(results []*RunResult, workspace string, iteration int) error {
	var withSkill, baseline []*RunResult
	for _, r := range results {
		switch r.Config {
		case "with_skill":
			withSkill = append(withSkill, r)
		case "baseline":
			baseline = append(baseline, r)
		}
	}

	ws := aggregateRuns(withSkill)
	bs := aggregateRuns(baseline)

	bf := BenchmarkFile{
		GeneratedAt: time.Now(),
	}
	bf.RunSummary.WithSkill = ws
	bf.RunSummary.Baseline = bs
	bf.RunSummary.Delta.PassRate = ws.PassRate.Mean - bs.PassRate.Mean
	bf.RunSummary.Delta.TimeSeconds = ws.TimeSeconds.Mean - bs.TimeSeconds.Mean
	bf.RunSummary.Delta.Tokens = ws.Tokens.Mean - bs.Tokens.Mean

	path := fmt.Sprintf("%s/benchmark.json", iterationPath(workspace, iteration))
	data, err := json.MarshalIndent(bf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// aggregateRuns computes mean and stddev across run results.
func aggregateRuns(results []*RunResult) RunSummary {
	// ponytail: handle empty slice gracefully
	if len(results) == 0 {
		return RunSummary{}
	}

	var passRates, times, tokens []float64
	for _, r := range results {
		if r.Grading != nil {
			passRates = append(passRates, r.Grading.Summary.PassRate)
		}
		if r.Timing != nil {
			times = append(times, float64(r.Timing.DurationMs)/1000.0)
			tokens = append(tokens, float64(r.Timing.TotalTokens))
		}
	}

	return RunSummary{
		PassRate:    Stats{Mean: mean(passRates), Stddev: stddev(passRates)},
		TimeSeconds: Stats{Mean: mean(times), Stddev: stddev(times)},
		Tokens:      Stats{Mean: mean(tokens), Stddev: stddev(tokens)},
	}
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func stddev(vals []float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	m := mean(vals)
	sumSq := 0.0
	for _, v := range vals {
		d := v - m
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(vals)))
}
