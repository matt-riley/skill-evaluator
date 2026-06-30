package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestConfigMerge(t *testing.T) {
	tests := []struct {
		name string
		dst  *Config
		src  *Config
		want *Config
	}{
		{
			name: "global defaults preserved when src is empty",
			dst:  &Config{Defaults: DefaultsConfig{Agent: "pi", Model: "base"}, Judge: JudgeConfig{Agent: "pi", Model: "judge"}},
			src:  &Config{},
			want: &Config{Defaults: DefaultsConfig{Agent: "pi", Model: "base"}, Judge: JudgeConfig{Agent: "pi", Model: "judge"}},
		},
		{
			name: "skill overrides override non-empty values",
			dst:  &Config{Defaults: DefaultsConfig{Agent: "pi", Model: "base"}, Judge: JudgeConfig{Agent: "pi", Model: "judge"}},
			src:  &Config{Defaults: DefaultsConfig{Agent: "claude", Model: "sonnet"}, Judge: JudgeConfig{Agent: "gpt", Model: "4o"}},
			want: &Config{Defaults: DefaultsConfig{Agent: "claude", Model: "sonnet"}, Judge: JudgeConfig{Agent: "gpt", Model: "4o"}},
		},
		{
			name: "empty skill fields do not clobber defaults",
			dst:  &Config{Defaults: DefaultsConfig{Agent: "pi", Model: "base"}, Judge: JudgeConfig{Agent: "pi", Model: "judge"}},
			src:  &Config{Defaults: DefaultsConfig{Model: "override"}},
			want: &Config{Defaults: DefaultsConfig{Agent: "pi", Model: "override"}, Judge: JudgeConfig{Agent: "pi", Model: "judge"}},
		},
		{
			name: "models slice is replaced when src non-empty",
			dst:  &Config{Models: []ModelConfig{{Agent: "pi", Model: "base"}}},
			src:  &Config{Models: []ModelConfig{{Agent: "claude", Model: "sonnet"}}},
			want: &Config{Models: []ModelConfig{{Agent: "claude", Model: "sonnet"}}},
		},
		{
			name: "empty models slice does not clobber",
			dst:  &Config{Models: []ModelConfig{{Agent: "pi", Model: "base"}}},
			src:  &Config{Judge: JudgeConfig{Agent: "gpt"}},
			want: &Config{Models: []ModelConfig{{Agent: "pi", Model: "base"}}, Judge: JudgeConfig{Agent: "gpt"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mergeConfig(tt.dst, tt.src)
			if !reflect.DeepEqual(tt.dst, tt.want) {
				t.Errorf("got %+v, want %+v", tt.dst, tt.want)
			}
		})
	}
}

func TestConfigModelKey(t *testing.T) {
	tests := []struct {
		name string
		m    ModelConfig
		want string
	}{
		{name: "agent and model", m: ModelConfig{Agent: "pi", Model: "claude-sonnet"}, want: "pi-claude-sonnet"},
		{name: "agent only", m: ModelConfig{Agent: "pi"}, want: "pi"},
		{name: "spaces replaced", m: ModelConfig{Agent: "some agent", Model: "some model"}, want: "some-agent-some-model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.Key(); got != tt.want {
				t.Errorf("Key() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadConfigValidation(t *testing.T) {
	writeConfig := func(t *testing.T, dir, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, ".skill-eval.yaml"), []byte(content), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}

	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name: "valid config loads",
			config: `defaults:
  agent: pi
  model: claude-sonnet
judge:
  agent: claude
  model: gpt-4o-mini
models:
  - agent: codex
    model: o1
`,
		},
		{
			name: "unknown agent name",
			config: `defaults:
  agent: badagent
`,
			wantErr: "badagent",
		},
		{
			name: "missing defaults.agent",
			config: `defaults:
  model: claude
`,
			wantErr: "agent",
		},
		{
			name: "unknown top-level key",
			config: `defaults:
  agent: pi
unknown_key: value
`,
			wantErr: "unknown_key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)

			skillDir := t.TempDir()
			writeConfig(t, skillDir, tt.config)

			cfg, err := LoadConfig(skillDir)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg == nil {
				t.Fatal("expected config, got nil")
			}
		})
	}
}

func TestConfigResolveModels(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *Config
		cliModels []ModelConfig
		want      []ModelConfig
	}{
		{
			name:      "CLI wins over config",
			cfg:       &Config{Models: []ModelConfig{{Agent: "pi", Model: "base"}}},
			cliModels: []ModelConfig{{Agent: "claude", Model: "sonnet"}},
			want:      []ModelConfig{{Agent: "claude", Model: "sonnet"}},
		},
		{
			name: "config models win over defaults",
			cfg:  &Config{Defaults: DefaultsConfig{Agent: "pi", Model: "base"}, Models: []ModelConfig{{Agent: "claude", Model: "sonnet"}}},
			want: []ModelConfig{{Agent: "claude", Model: "sonnet"}},
		},
		{
			name: "defaults when everything empty",
			cfg:  &Config{Defaults: DefaultsConfig{Agent: "pi", Model: "base"}},
			want: []ModelConfig{{Agent: "pi", Model: "base"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveModels(tt.cfg, tt.cliModels)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
