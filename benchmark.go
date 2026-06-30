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
		Models:      map[string]ModelBenchmark{},
	}

	// Best/worst rank by absolute with-skill pass rate (matches the report's
	// "Best performer" label) and only when there are >=2 models, so a
	// single-model run doesn't label the same model as both best and worst.
	bestRate := -1.0
	worstRate := 2.0
	rankModels := len(byModel) >= 2
	for mk, rs := range byModel {
		ws, bs := splitAndAggregate(rs)
		mb := ModelBenchmark{WithSkill: ws, Baseline: bs, Delta: computeDelta(ws, bs)}
		bf.Models[mk] = mb

		if rankModels {
			rate := mb.WithSkill.PassRate.Mean
			if rate > bestRate {
				bestRate = rate
				bf.BestModel = mk
			}
			if rate < worstRate {
				worstRate = rate
				bf.WorstModel = mk
			}
		}
	}

	// Populate the top-level run_summary as the cross-model aggregate so
	// consumers reading run_summary see real numbers instead of zeros.
	wsAgg, bsAgg := aggregateAcrossModels(bf.Models)
	bf.RunSummary.WithSkill = wsAgg
	bf.RunSummary.Baseline = bsAgg
	bf.RunSummary.Delta = computeDelta(wsAgg, bsAgg)

	prevIter, prev, err := loadPreviousBenchmark(workspace, iteration)
	if err != nil {
		return err
	}
	if prev != nil {
		bf.PreviousIteration = prevIter
		bf.IterationDelta = subtractDeltas(
			averageDelta(modelDeltas(bf.Models)),
			averageDelta(modelDeltas(prev.allModels())),
		)
	}

	if err := ensureDir(iterationPath(workspace, iteration)); err != nil {
		return err
	}
	path := fmt.Sprintf("%s/benchmark.json", iterationPath(workspace, iteration))
	data, err := json.MarshalIndent(bf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// loadPreviousBenchmark walks backward from currentIter-1 and returns the first
// benchmark.json found, or (0, nil, nil) if none exists.
func loadPreviousBenchmark(workspace string, currentIter int) (int, *BenchmarkFile, error) {
	for i := currentIter - 1; i >= 1; i-- {
		path := fmt.Sprintf("%s/benchmark.json", iterationPath(workspace, i))
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return 0, nil, err
		}
		var bf BenchmarkFile
		if err := json.Unmarshal(data, &bf); err != nil {
			return 0, nil, err
		}
		return i, &bf, nil
	}
	return 0, nil, nil
}

// allModels returns the per-model benchmarks: Models if present (current
// format), else a single-entry map synthesized from the legacy RunSummary
// field, so old benchmark.json files on disk still feed the delta calc.
func (bf *BenchmarkFile) allModels() map[string]ModelBenchmark {
	if len(bf.Models) > 0 {
		return bf.Models
	}
	return map[string]ModelBenchmark{"_": {
		WithSkill: bf.RunSummary.WithSkill,
		Baseline:  bf.RunSummary.Baseline,
		Delta:     bf.RunSummary.Delta,
	}}
}

// modelDeltas extracts the delta for each model in a ModelBenchmark map.
func modelDeltas(models map[string]ModelBenchmark) []Delta {
	deltas := make([]Delta, 0, len(models))
	for _, mb := range models {
		deltas = append(deltas, mb.Delta)
	}
	return deltas
}

// averageDelta returns the mean of a slice of deltas.
func averageDelta(deltas []Delta) Delta {
	if len(deltas) == 0 {
		return Delta{}
	}
	var sum Delta
	for _, d := range deltas {
		sum.PassRate += d.PassRate
		sum.TimeSeconds += d.TimeSeconds
		sum.Tokens += d.Tokens
	}
	n := float64(len(deltas))
	return Delta{
		PassRate:    sum.PassRate / n,
		TimeSeconds: sum.TimeSeconds / n,
		Tokens:      sum.Tokens / n,
	}
}

// subtractDeltas returns a - b.
func subtractDeltas(a, b Delta) *Delta {
	return &Delta{
		PassRate:    a.PassRate - b.PassRate,
		TimeSeconds: a.TimeSeconds - b.TimeSeconds,
		Tokens:      a.Tokens - b.Tokens,
	}
}

// aggregateAcrossModels averages each model's WithSkill/Baseline means into a
// single RunSummary, treating each model as one sample. Stddev here is the
// cross-model spread of means, not a within-model statistic.
func aggregateAcrossModels(models map[string]ModelBenchmark) (RunSummary, RunSummary) {
	var wsPR, wsT, wsTok, bsPR, bsT, bsTok []float64
	for _, mb := range models {
		wsPR = append(wsPR, mb.WithSkill.PassRate.Mean)
		wsT = append(wsT, mb.WithSkill.TimeSeconds.Mean)
		wsTok = append(wsTok, mb.WithSkill.Tokens.Mean)
		bsPR = append(bsPR, mb.Baseline.PassRate.Mean)
		bsT = append(bsT, mb.Baseline.TimeSeconds.Mean)
		bsTok = append(bsTok, mb.Baseline.Tokens.Mean)
	}
	return RunSummary{
			PassRate:    Stats{Mean: mean(wsPR), Stddev: stddev(wsPR)},
			TimeSeconds: Stats{Mean: mean(wsT), Stddev: stddev(wsT)},
			Tokens:      Stats{Mean: mean(wsTok), Stddev: stddev(wsTok)},
		}, RunSummary{
			PassRate:    Stats{Mean: mean(bsPR), Stddev: stddev(bsPR)},
			TimeSeconds: Stats{Mean: mean(bsT), Stddev: stddev(bsT)},
			Tokens:      Stats{Mean: mean(bsTok), Stddev: stddev(bsTok)},
		}
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
