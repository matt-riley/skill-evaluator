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
	// Group results by model
	byModel := map[string][]*RunResult{}
	for _, r := range results {
		mk := r.Model
		if mk == "" {
			mk = "_" // legacy single-model
		}
		byModel[mk] = append(byModel[mk], r)
	}

	bf := BenchmarkFile{
		GeneratedAt: time.Now(),
	}

	if len(byModel) == 1 {
		// Single model (or legacy) — use flat summary format
		for _, rs := range byModel {
			bf.RunSummary.WithSkill, bf.RunSummary.Baseline = splitAndAggregate(rs)
			bf.RunSummary.Delta = computeDelta(bf.RunSummary.WithSkill, bf.RunSummary.Baseline)
		}
	} else {
		// Multi-model — use models map
		bf.Models = map[string]ModelBenchmark{}
		bestDelta := -999.0
		worstDelta := 999.0
		for mk, rs := range byModel {
			ws, bs := splitAndAggregate(rs)
			mb := ModelBenchmark{WithSkill: ws, Baseline: bs, Delta: computeDelta(ws, bs)}
			bf.Models[mk] = mb

			if mb.Delta.PassRate > bestDelta {
				bestDelta = mb.Delta.PassRate
				bf.BestModel = mk
			}
			if mb.Delta.PassRate < worstDelta {
				worstDelta = mb.Delta.PassRate
				bf.WorstModel = mk
			}
		}
	}

	path := fmt.Sprintf("%s/benchmark.json", iterationPath(workspace, iteration))
	data, err := json.MarshalIndent(bf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// splitAndAggregate splits results into with_skill and baseline, then aggregates each.
func splitAndAggregate(results []*RunResult) (RunSummary, RunSummary) {
	var withSkill, baseline []*RunResult
	for _, r := range results {
		switch r.Config {
		case "with_skill":
			withSkill = append(withSkill, r)
		case "baseline":
			baseline = append(baseline, r)
		}
	}
	return aggregateRuns(withSkill), aggregateRuns(baseline)
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

func computeDelta(withSkill, baseline RunSummary) Delta {
	return Delta{
		PassRate:    withSkill.PassRate.Mean - baseline.PassRate.Mean,
		TimeSeconds: withSkill.TimeSeconds.Mean - baseline.TimeSeconds.Mean,
		Tokens:      withSkill.Tokens.Mean - baseline.Tokens.Mean,
	}
}
