package main

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// skillFrontmatter is the spec's frontmatter schema, typed for fields
// that activation evals and other tooling need (name, description).
// Spec: https://agentskills.io/specification (rules snapshot 2026-07-01).
type skillFrontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`
	AllowedTools  string            `yaml:"allowed-tools"`
	Version       string            `yaml:"version"`
}

// parseSkillMD splits raw SKILL.md data into frontmatter (map + typed struct)
// and body. Frontmatter is the YAML between the first "---" and the next line
// that is exactly "---". Returns an error if frontmatter is present but
// malformed. When no frontmatter is present, fm and sf are zero-valued and
// body is the content as-is.
//
// Extracted from Plan 009's cmd_validate.go for reuse by activation evals
// (Plan 013). When Plan 009 lands fully, this should be consolidated.
func parseSkillMD(data []byte) (fm map[string]any, sf skillFrontmatter, body string, err error) {
	content := string(data)
	if !strings.HasPrefix(content, "---\n") {
		return nil, skillFrontmatter{}, content, nil // no frontmatter
	}

	rest := content[4:] // skip "---\n"

	// Special case: empty frontmatter (content is "---\n---\n...")
	if strings.HasPrefix(rest, "---\n") {
		fm, err = parseFrontmatterRaw("")
		return fm, skillFrontmatter{}, rest[4:], err
	}
	// Special case: empty frontmatter at EOF (content is "---\n---")
	if rest == "---" {
		fm, err = parseFrontmatterRaw("")
		return fm, skillFrontmatter{}, "", err
	}

	idx := strings.Index(rest, "\n---\n")
	if idx == -1 {
		if strings.HasSuffix(rest, "\n---") {
			fmRaw := rest[:len(rest)-4]
			fm, err = parseFrontmatterRaw(fmRaw)
			if err != nil {
				return nil, skillFrontmatter{}, "", err
			}
			sf = parseTypedFrontmatter(fmRaw)
			return fm, sf, "", nil
		}
		return nil, skillFrontmatter{}, "", fmt.Errorf("unterminated frontmatter: missing closing ---")
	}

	fmRaw := rest[:idx]
	bodyStart := idx + 5 // len("\n---\n")
	body = rest[bodyStart:]

	fm, err = parseFrontmatterRaw(fmRaw)
	if err != nil {
		return nil, skillFrontmatter{}, "", err
	}
	sf = parseTypedFrontmatter(fmRaw)
	return fm, sf, body, nil
}

// parseFrontmatterRaw parses the raw string between --- markers into a map.
func parseFrontmatterRaw(raw string) (map[string]any, error) {
	m := make(map[string]any)
	if err := yaml.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("frontmatter is not valid YAML: %w", err)
	}
	return m, nil
}

// parseTypedFrontmatter parses the raw string into the typed struct.
// Errors are intentionally swallowed — the caller already validated YAML
// via parseFrontmatterRaw; if the typed parse fails, sf stays zero-valued.
func parseTypedFrontmatter(raw string) skillFrontmatter {
	var sf skillFrontmatter
	_ = yaml.Unmarshal([]byte(raw), &sf)
	return sf
}
