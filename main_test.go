package main

import (
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
