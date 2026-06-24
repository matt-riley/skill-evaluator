package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the tool's configuration, loaded from YAML.
type Config struct {
	Defaults DefaultsConfig `yaml:"defaults"`
	Judge    JudgeConfig    `yaml:"judge"`
}

// DefaultsConfig is the default agent + model for running evals.
type DefaultsConfig struct {
	Agent string `yaml:"agent"`
	Model string `yaml:"model"`
}

// JudgeConfig is the configuration for the grading judge.
type JudgeConfig struct {
	Agent string `yaml:"agent"`
	Model string `yaml:"model"`
}

// LoadConfig loads config from global then skill-level, merging.
// Skill-level overrides take precedence.
func LoadConfig(skillDir string) (*Config, error) {
	cfg := &Config{
		Defaults: DefaultsConfig{Agent: "pi"},
		Judge:    JudgeConfig{Agent: "pi"},
	}

	// Load global config
	home, err := os.UserHomeDir()
	if err == nil {
		globalPath := filepath.Join(home, ".config", "skill-eval", "config.yaml")
		if data, err := os.ReadFile(globalPath); err == nil {
			var globalCfg Config
			if err := yaml.Unmarshal(data, &globalCfg); err != nil {
				return nil, fmt.Errorf("global config %s: %w", globalPath, err)
			}
			mergeConfig(cfg, &globalCfg)
		}
	}

	// Load skill-level overrides
	if skillDir != "" {
		skillCfgPath := filepath.Join(skillDir, ".skill-eval.yaml")
		if data, err := os.ReadFile(skillCfgPath); err == nil {
			var skillCfg Config
			if err := yaml.Unmarshal(data, &skillCfg); err != nil {
				return nil, fmt.Errorf("skill config %s: %w", skillCfgPath, err)
			}
			mergeConfig(cfg, &skillCfg)
		}
	}

	return cfg, nil
}

// mergeConfig copies non-zero values from src into dst.
func mergeConfig(dst, src *Config) {
	if src.Defaults.Agent != "" {
		dst.Defaults.Agent = src.Defaults.Agent
	}
	if src.Defaults.Model != "" {
		dst.Defaults.Model = src.Defaults.Model
	}
	if src.Judge.Agent != "" {
		dst.Judge.Agent = src.Judge.Agent
	}
	if src.Judge.Model != "" {
		dst.Judge.Model = src.Judge.Model
	}
}
