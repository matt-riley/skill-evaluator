package main

import (
	"reflect"
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
