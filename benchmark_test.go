package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
	"time"
)

func TestBenchmarkMean(t *testing.T) {
	if got := mean(nil); got != 0 {
		t.Errorf("mean(nil) = %v, want 0", got)
	}
	if got := mean([]float64{1, 2, 3}); got != 2 {
		t.Errorf("mean(...) = %v, want 2", got)
	}
}

func TestBenchmarkStddev(t *testing.T) {
	if got := stddev([]float64{42}); got != 0 {
		t.Errorf("stddev(<2) = %v, want 0", got)
	}
	if got := stddev(nil); got != 0 {
		t.Errorf("stddev(nil) = %v, want 0", got)
	}
	// stddev of [1, 3] around mean 2 is sqrt(((1)^2 + (1)^2)/2) = 1
	if got := stddev([]float64{1, 3}); got != 1 {
		t.Errorf("stddev([1,3]) = %v, want 1", got)
	}
}

func TestBenchmarkComputeDelta(t *testing.T) {
	ws := RunSummary{
		PassRate:    Stats{Mean: 0.9},
		TimeSeconds: Stats{Mean: 2.0},
		Tokens:      Stats{Mean: 100},
	}
	bs := RunSummary{
		PassRate:    Stats{Mean: 0.7},
		TimeSeconds: Stats{Mean: 1.5},
		Tokens:      Stats{Mean: 80},
	}
	delta := computeDelta(ws, bs)
	if math.Abs(delta.PassRate-0.2) > 1e-9 {
		t.Errorf("PassRate delta = %v, want 0.2", delta.PassRate)
	}
	if math.Abs(delta.TimeSeconds-0.5) > 1e-9 {
		t.Errorf("TimeSeconds delta = %v, want 0.5", delta.TimeSeconds)
	}
	if math.Abs(delta.Tokens-20) > 1e-9 {
		t.Errorf("Tokens delta = %v, want 20", delta.Tokens)
	}
}

func TestBenchmarkAggregateRuns(t *testing.T) {
	t.Run("nil grading and timing handled gracefully", func(t *testing.T) {
		got := aggregateRuns([]*RunResult{{}, {}})
		want := RunSummary{}
		if !cmpRunSummary(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("aggregates pass rates and timing", func(t *testing.T) {
		got := aggregateRuns([]*RunResult{
			{Grading: &GradingFile{Summary: GradingSummary{PassRate: 0.5}}, Timing: &TimingData{DurationMs: 1000, TotalTokens: 50}},
			{Grading: &GradingFile{Summary: GradingSummary{PassRate: 1.0}}, Timing: &TimingData{DurationMs: 3000, TotalTokens: 150}},
		})
		if math.Abs(got.PassRate.Mean-0.75) > 1e-9 {
			t.Errorf("PassRate mean = %v, want 0.75", got.PassRate.Mean)
		}
		if math.Abs(got.TimeSeconds.Mean-2.0) > 1e-9 {
			t.Errorf("TimeSeconds mean = %v, want 2.0", got.TimeSeconds.Mean)
		}
		if math.Abs(got.Tokens.Mean-100) > 1e-9 {
			t.Errorf("Tokens mean = %v, want 100", got.Tokens.Mean)
		}
	})
}

func TestSubtractDeltas(t *testing.T) {
	a := Delta{PassRate: 0.8, TimeSeconds: 2.0, Tokens: 100}
	b := Delta{PassRate: 0.6, TimeSeconds: 1.5, Tokens: 80}
	got := subtractDeltas(a, b)
	if math.Abs(got.PassRate-0.2) > 1e-9 || math.Abs(got.TimeSeconds-0.5) > 1e-9 || math.Abs(got.Tokens-20) > 1e-9 {
		t.Errorf("subtractDeltas(a, b) = %+v, want {0.2 0.5 20}", *got)
	}

	got = subtractDeltas(b, a)
	if math.Abs(got.PassRate+0.2) > 1e-9 || math.Abs(got.TimeSeconds+0.5) > 1e-9 || math.Abs(got.Tokens+20) > 1e-9 {
		t.Errorf("subtractDeltas(b, a) = %+v, want {-0.2 -0.5 -20}", *got)
	}
}

func TestLoadPreviousBenchmark(t *testing.T) {
	dir := t.TempDir()

	// No previous iteration => nil.
	iter, prev, err := loadPreviousBenchmark(dir, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prev != nil || iter != 0 {
		t.Fatalf("expected no previous benchmark, got iter=%d prev=%v", iter, prev)
	}

	// Create iteration-1 and iteration-2 benchmark files.
	mustWriteBenchmark(t, dir, 1, &BenchmarkFile{GeneratedAt: time.Now()})
	mustWriteBenchmark(t, dir, 2, &BenchmarkFile{GeneratedAt: time.Now()})

	// Should find iteration-2 first when current is 3.
	iter, prev, err = loadPreviousBenchmark(dir, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prev == nil {
		t.Fatal("expected previous benchmark, got nil")
	}
	if iter != 2 {
		t.Fatalf("expected previous iteration 2, got %d", iter)
	}

	// Should skip missing iteration-2 and find iteration-1.
	if err := os.Remove(fmt.Sprintf("%s/iteration-2/benchmark.json", dir)); err != nil {
		t.Fatalf("remove: %v", err)
	}
	iter, prev, err = loadPreviousBenchmark(dir, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prev == nil {
		t.Fatal("expected previous benchmark, got nil")
	}
	if iter != 1 {
		t.Fatalf("expected previous iteration 1, got %d", iter)
	}
}

func TestIterationDeltaSingleModel(t *testing.T) {
	dir := t.TempDir()
	mustWriteBenchmark(t, dir, 1, &BenchmarkFile{
		RunSummary: struct {
			WithSkill RunSummary `json:"with_skill"`
			Baseline  RunSummary `json:"baseline"`
			Delta     Delta      `json:"delta"`
		}{
			Delta: Delta{PassRate: 0.1, TimeSeconds: 0.2, Tokens: 10},
		},
		GeneratedAt: time.Now(),
	})

	current := []*RunResult{
		{Config: "with_skill", Grading: &GradingFile{Summary: GradingSummary{PassRate: 0.9}}, Timing: &TimingData{DurationMs: 2000, TotalTokens: 110}},
		{Config: "baseline", Grading: &GradingFile{Summary: GradingSummary{PassRate: 0.6}}, Timing: &TimingData{DurationMs: 1500, TotalTokens: 80}},
	}
	if err := computeBenchmark(current, dir, 2); err != nil {
		t.Fatalf("computeBenchmark failed: %v", err)
	}

	bf := mustReadBenchmark(t, dir, 2)
	if bf.PreviousIteration != 1 {
		t.Fatalf("PreviousIteration = %d, want 1", bf.PreviousIteration)
	}
	if bf.IterationDelta == nil {
		t.Fatal("IterationDelta is nil, want non-nil")
	}
	// Current delta: 0.9-0.6=0.3, 2.0-1.5=0.5, 110-80=30
	// Previous delta: 0.1, 0.2, 10
	// Iteration delta: 0.2, 0.3, 20
	if math.Abs(bf.IterationDelta.PassRate-0.2) > 1e-9 || math.Abs(bf.IterationDelta.TimeSeconds-0.3) > 1e-9 || math.Abs(bf.IterationDelta.Tokens-20) > 1e-9 {
		t.Fatalf("IterationDelta = %+v, want {0.2 0.3 20}", *bf.IterationDelta)
	}
}

func TestIterationDeltaNoPrevious(t *testing.T) {
	dir := t.TempDir()
	current := []*RunResult{
		{Config: "with_skill", Grading: &GradingFile{Summary: GradingSummary{PassRate: 0.8}}, Timing: &TimingData{DurationMs: 1000, TotalTokens: 100}},
		{Config: "baseline", Grading: &GradingFile{Summary: GradingSummary{PassRate: 0.5}}, Timing: &TimingData{DurationMs: 800, TotalTokens: 70}},
	}
	if err := computeBenchmark(current, dir, 1); err != nil {
		t.Fatalf("computeBenchmark failed: %v", err)
	}

	bf := mustReadBenchmark(t, dir, 1)
	if bf.PreviousIteration != 0 {
		t.Fatalf("PreviousIteration = %d, want 0", bf.PreviousIteration)
	}
	if bf.IterationDelta != nil {
		t.Fatalf("IterationDelta = %+v, want nil", bf.IterationDelta)
	}
}

func TestBestWorstModelRanking(t *testing.T) {
	// Two models: A passes more with the skill (best), B passes less (worst),
	// even though B has the bigger delta — ranking is by absolute pass rate.
	results := []*RunResult{
		{Model: "A", Config: "with_skill", Grading: &GradingFile{Summary: GradingSummary{PassRate: 0.9}}},
		{Model: "A", Config: "baseline", Grading: &GradingFile{Summary: GradingSummary{PassRate: 0.85}}},
		{Model: "B", Config: "with_skill", Grading: &GradingFile{Summary: GradingSummary{PassRate: 0.4}}},
		{Model: "B", Config: "baseline", Grading: &GradingFile{Summary: GradingSummary{PassRate: 0.1}}},
	}
	dir := t.TempDir()
	if err := computeBenchmark(results, dir, 1); err != nil {
		t.Fatalf("computeBenchmark: %v", err)
	}
	bf := mustReadBenchmark(t, dir, 1)
	if bf.BestModel != "A" {
		t.Errorf("BestModel = %q, want A", bf.BestModel)
	}
	if bf.WorstModel != "B" {
		t.Errorf("WorstModel = %q, want B", bf.WorstModel)
	}
}

func TestBestWorstModelSingleModelUnset(t *testing.T) {
	// A single model must not be labeled as both best and worst.
	results := []*RunResult{
		{Model: "", Config: "with_skill", Grading: &GradingFile{Summary: GradingSummary{PassRate: 0.9}}},
		{Model: "", Config: "baseline", Grading: &GradingFile{Summary: GradingSummary{PassRate: 0.6}}},
	}
	dir := t.TempDir()
	if err := computeBenchmark(results, dir, 1); err != nil {
		t.Fatalf("computeBenchmark: %v", err)
	}
	bf := mustReadBenchmark(t, dir, 1)
	if bf.BestModel != "" || bf.WorstModel != "" {
		t.Errorf("single model: Best=%q Worst=%q, want both empty", bf.BestModel, bf.WorstModel)
	}
}

func TestRunSummaryAggregate(t *testing.T) {
	// Top-level run_summary must be the cross-model average, not zeros.
	results := []*RunResult{
		{Model: "A", Config: "with_skill", Grading: &GradingFile{Summary: GradingSummary{PassRate: 0.8}}, Timing: &TimingData{DurationMs: 1000, TotalTokens: 100}},
		{Model: "A", Config: "baseline", Grading: &GradingFile{Summary: GradingSummary{PassRate: 0.6}}, Timing: &TimingData{DurationMs: 800, TotalTokens: 80}},
		{Model: "B", Config: "with_skill", Grading: &GradingFile{Summary: GradingSummary{PassRate: 0.6}}, Timing: &TimingData{DurationMs: 2000, TotalTokens: 300}},
		{Model: "B", Config: "baseline", Grading: &GradingFile{Summary: GradingSummary{PassRate: 0.4}}, Timing: &TimingData{DurationMs: 1800, TotalTokens: 250}},
	}
	dir := t.TempDir()
	if err := computeBenchmark(results, dir, 1); err != nil {
		t.Fatalf("computeBenchmark: %v", err)
	}
	bf := mustReadBenchmark(t, dir, 1)
	// Average with-skill pass rate across A(0.8) and B(0.6) = 0.7
	if math.Abs(bf.RunSummary.WithSkill.PassRate.Mean-0.7) > 1e-9 {
		t.Errorf("run_summary.with_skill.pass_rate.mean = %v, want 0.7", bf.RunSummary.WithSkill.PassRate.Mean)
	}
	// Average baseline = (0.6+0.4)/2 = 0.5
	if math.Abs(bf.RunSummary.Baseline.PassRate.Mean-0.5) > 1e-9 {
		t.Errorf("run_summary.baseline.pass_rate.mean = %v, want 0.5", bf.RunSummary.Baseline.PassRate.Mean)
	}
	// Delta = 0.7 - 0.5 = 0.2
	if math.Abs(bf.RunSummary.Delta.PassRate-0.2) > 1e-9 {
		t.Errorf("run_summary.delta.pass_rate = %v, want 0.2", bf.RunSummary.Delta.PassRate)
	}
}

func mustWriteBenchmark(t *testing.T, dir string, iter int, bf *BenchmarkFile) {
	t.Helper()
	path := iterationPath(dir, iter)
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, err := json.MarshalIndent(bf, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(fmt.Sprintf("%s/benchmark.json", path), data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func mustReadBenchmark(t *testing.T, dir string, iter int) *BenchmarkFile {
	t.Helper()
	data, err := os.ReadFile(fmt.Sprintf("%s/benchmark.json", iterationPath(dir, iter)))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var bf BenchmarkFile
	if err := json.Unmarshal(data, &bf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &bf
}

func cmpRunSummary(a, b RunSummary) bool {
	return a.PassRate == b.PassRate && a.TimeSeconds == b.TimeSeconds && a.Tokens == b.Tokens
}
