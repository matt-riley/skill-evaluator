package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseModels(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []ModelConfig
		wantErr bool
	}{
		{
			name: "empty string returns nil",
			raw:  "",
			want: nil,
		},
		{
			name: "single agent:model pair",
			raw:  "pi:claude-sonnet",
			want: []ModelConfig{{Agent: "pi", Model: "claude-sonnet"}},
		},
		{
			name: "multiple pairs",
			raw:  "pi:sonnet,claude:opus,codex",
			want: []ModelConfig{{Agent: "pi", Model: "sonnet"}, {Agent: "claude", Model: "opus"}, {Agent: "codex"}},
		},
		{
			name: "missing model is allowed",
			raw:  "claude",
			want: []ModelConfig{{Agent: "claude"}},
		},
		{
			name: "whitespace trimmed",
			raw:  "  pi : sonnet  ,  claude  ",
			want: []ModelConfig{{Agent: "pi", Model: "sonnet"}, {Agent: "claude"}},
		},
		{
			name:    "empty list after trimming returns error",
			raw:     "  ,  ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseModels(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("got %d models, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("model[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCmdBenchmarkReadsModelKeyedPaths(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	evalsDir := filepath.Join(skillDir, "evals")
	if err := os.MkdirAll(evalsDir, 0o755); err != nil {
		t.Fatalf("mkdir evals: %v", err)
	}
	evalsJSON := `{"skill_name":"test","evals":[{"id":1,"prompt":"p","expected_output":"e","assertions":["file_exists:x"]}]}`
	if err := os.WriteFile(filepath.Join(evalsDir, "evals.json"), []byte(evalsJSON), 0o644); err != nil {
		t.Fatalf("write evals.json: %v", err)
	}

	ws := workspacePath(skillDir)
	iter := 1

	// Build the model-keyed layout that config models produce when keys contain slashes.
	mk := "pi-deepseek/deepseek-v4-flash"
	for _, config := range []string{"with_skill", "baseline"} {
		base := filepath.Join(evalPath(ws, iter, 1, mk), config)
		if err := os.MkdirAll(base, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", config, err)
		}
		gf := &GradingFile{Summary: GradingSummary{Passed: 1, Total: 1, PassRate: 1}}
		data, _ := json.MarshalIndent(gf, "", "  ")
		if err := os.WriteFile(filepath.Join(base, "grading.json"), data, 0o644); err != nil {
			t.Fatalf("write grading: %v", err)
		}
		td := &TimingData{DurationMs: 1000, TotalTokens: 50}
		data, _ = json.MarshalIndent(td, "", "  ")
		if err := os.WriteFile(filepath.Join(base, "timing.json"), data, 0o644); err != nil {
			t.Fatalf("write timing: %v", err)
		}
	}

	if err := os.Chdir(skillDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if err := cmdBenchmark([]string{"--models", "pi:deepseek/deepseek-v4-flash"}); err != nil {
		t.Fatalf("cmdBenchmark failed: %v", err)
	}

	bf := mustReadBenchmark(t, ws, iter)
	mb, ok := bf.Models[mk]
	if !ok {
		t.Fatalf("missing model %q in %+v", mk, bf.Models)
	}
	if mb.WithSkill.PassRate.Mean != 1 || mb.Baseline.PassRate.Mean != 1 {
		t.Fatalf("unexpected pass rates: with_skill=%v baseline=%v", mb.WithSkill.PassRate.Mean, mb.Baseline.PassRate.Mean)
	}
	if mb.WithSkill.Tokens.Mean != 50 || mb.Baseline.Tokens.Mean != 50 {
		t.Fatalf("unexpected tokens: with_skill=%v baseline=%v", mb.WithSkill.Tokens.Mean, mb.Baseline.Tokens.Mean)
	}
}

func TestVerboseFlagParsed(t *testing.T) {
	tests := []struct {
		name        string
		in          []string
		wantSubcmd  string
		wantArgs    []string
		wantVerbose bool
	}{
		{
			name:        "long flag before subcommand",
			in:          []string{"--verbose", "run", "--eval", "1"},
			wantSubcmd:  "run",
			wantArgs:    []string{"--eval", "1"},
			wantVerbose: true,
		},
		{
			name:        "short flag before subcommand",
			in:          []string{"-v", "grade"},
			wantSubcmd:  "grade",
			wantArgs:    nil,
			wantVerbose: true,
		},
		{
			name:        "verbose after subcommand",
			in:          []string{"loop", "--verbose"},
			wantSubcmd:  "loop",
			wantArgs:    nil,
			wantVerbose: true,
		},
		{
			name:        "no flag",
			in:          []string{"benchmark"},
			wantSubcmd:  "benchmark",
			wantArgs:    nil,
			wantVerbose: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subcmd, args, verbose := parseGlobalArgs(tt.in)
			if subcmd != tt.wantSubcmd {
				t.Errorf("subcmd = %q, want %q", subcmd, tt.wantSubcmd)
			}
			if !reflect.DeepEqual(args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
			}
			if verbose != tt.wantVerbose {
				t.Errorf("verbose = %v, want %v", verbose, tt.wantVerbose)
			}
		})
	}
}
