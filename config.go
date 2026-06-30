package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

//go:embed schema/config-schema.json
var configSchemaJSON []byte

// Config holds the tool's configuration, loaded from YAML.
type Config struct {
	Defaults DefaultsConfig `yaml:"defaults"`
	Judge    JudgeConfig    `yaml:"judge"`
	Models   []ModelConfig  `yaml:"models,omitempty"`
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

// ModelConfig specifies an agent + optional model for a run configuration.
type ModelConfig struct {
	Agent string `yaml:"agent"`
	Model string `yaml:"model,omitempty"`
}

// valueAtPath walks a nested JSON-compatible value built from maps and slices.
func valueAtPath(root map[string]any, loc []string) (any, bool) {
	cur := any(root)
	for _, p := range loc {
		if p == "" {
			continue
		}
		switch v := cur.(type) {
		case map[string]any:
			nxt, ok := v[p]
			if !ok {
				return nil, false
			}
			cur = nxt
		case []any:
			idx, err := strconv.Atoi(p)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, false
			}
			cur = v[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

// firstValidationLocation returns the first nested validation error location.
func firstValidationLocation(err *jsonschema.ValidationError) []string {
	if len(err.InstanceLocation) > 0 {
		return err.InstanceLocation
	}
	for _, c := range err.Causes {
		if loc := firstValidationLocation(c); loc != nil {
			return loc
		}
	}
	return nil
}

// validateConfigYAML validates raw YAML config bytes against the embedded JSON schema.
func validateConfigYAML(data []byte, path string) error {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}

	jsonData, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}

	var value any
	if err := json.Unmarshal(jsonData, &value); err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}

	c := jsonschema.NewCompiler()
	var schemaDoc any
	if err := json.Unmarshal(configSchemaJSON, &schemaDoc); err != nil {
		return fmt.Errorf("config %s: invalid embedded schema: %w", path, err)
	}
	if err := c.AddResource("schema.json", schemaDoc); err != nil {
		return fmt.Errorf("config %s: invalid embedded schema: %w", path, err)
	}
	sch, err := c.Compile("schema.json")
	if err != nil {
		return fmt.Errorf("config %s: invalid embedded schema: %w", path, err)
	}
	if err := sch.Validate(value); err != nil {
		if verr, ok := err.(*jsonschema.ValidationError); ok {
			if loc := firstValidationLocation(verr); loc != nil {
				if val, ok := valueAtPath(raw, loc); ok {
					return fmt.Errorf("config %s: invalid value %v: %w", path, val, err)
				}
			}
		}
		return fmt.Errorf("config %s: %w", path, err)
	}
	return nil
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
		data, err := os.ReadFile(globalPath) // #nosec G304 -- globalPath is filepath.Join(home, ".config", "skill-eval", "config.yaml"), a fixed subpath under the user's home dir, not external input
		if err == nil {
			if err := validateConfigYAML(data, globalPath); err != nil {
				return nil, err
			}
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
		data, err := os.ReadFile(skillCfgPath) // #nosec G304 -- skillCfgPath is filepath.Join(skillDir, ".skill-eval.yaml") where skillDir was resolved by walking up from cwd to find SKILL.md
		if err == nil {
			if err := validateConfigYAML(data, skillCfgPath); err != nil {
				return nil, err
			}
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
	if len(src.Models) > 0 {
		dst.Models = src.Models
	}
}

// modelKey returns a kebab-cased key for a ModelConfig.
func (m ModelConfig) Key() string {
	if m.Model != "" {
		return strings.ReplaceAll(m.Agent+"-"+m.Model, " ", "-")
	}
	return m.Agent
}

// resolveModels returns the models to run against, defaulting to config defaults.
func resolveModels(cfg *Config, cliModels []ModelConfig) []ModelConfig {
	if len(cliModels) > 0 {
		return cliModels
	}
	if len(cfg.Models) > 0 {
		return cfg.Models
	}
	return []ModelConfig{{Agent: cfg.Defaults.Agent, Model: cfg.Defaults.Model}}
}
