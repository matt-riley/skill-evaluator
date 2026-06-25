package main

import (
	"math"
	"testing"
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

func cmpRunSummary(a, b RunSummary) bool {
	return a.PassRate == b.PassRate && a.TimeSeconds == b.TimeSeconds && a.Tokens == b.Tokens
}
