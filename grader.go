package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// gradeEval shells out to the judge agent to grade assertions against outputs.
func gradeEval(ctx context.Context, cfg *Config, eval Eval, workspace string, iteration int, modelKey string, configLabel string, cmdFn CmdBuilder) (*GradingFile, error) {
	outDir := filepath.Join(evalPath(workspace, iteration, eval.ID, modelKey), configLabel, "outputs")
	gradingPath := filepath.Join(evalPath(workspace, iteration, eval.ID, modelKey), configLabel, "grading.json")
	return gradeFromOutput(ctx, cfg, eval, outDir, gradingPath, fmt.Sprintf("eval %d (%s)", eval.ID, configLabel), cmdFn)
}

// gradeFromOutput evaluates deterministic assertions locally and sends the rest to the judge.
func gradeFromOutput(ctx context.Context, cfg *Config, eval Eval, outDir, gradingPath, contextLabel string, cmdFn CmdBuilder) (*GradingFile, error) {
	if len(eval.Assertions) == 0 {
		return nil, fmt.Errorf("eval %d has no assertions to grade", eval.ID)
	}

	outputContents := readOutputContents(outDir)

	results := make([]AssertionResult, len(eval.Assertions))
	llmAssertions := make([]string, 0, len(eval.Assertions))
	llmPositions := make([]int, 0, len(eval.Assertions))

	for i, a := range eval.Assertions {
		pa := parseAssertion(a)
		if pa.Type == MatcherLLM {
			llmAssertions = append(llmAssertions, a)
			llmPositions = append(llmPositions, i)
		} else {
			results[i] = evaluateMatcher(pa, outDir, outputContents)
		}
	}

	if len(llmAssertions) > 0 {
		llmEval := eval
		llmEval.Assertions = llmAssertions
		prompt := buildGradingPrompt(llmEval, outputContents)

		judgeAgent := cfg.Judge.Agent
		if judgeAgent == "" {
			judgeAgent = cfg.Defaults.Agent
		}
		judgeModel := cfg.Judge.Model
		if judgeModel == "" {
			judgeModel = cfg.Defaults.Model
		}

		logger.Debug("grading", "eval", eval.ID, "assertions", len(llmAssertions))
		if cmdFn == nil {
			cmdFn = buildAgentCmd
		}
		cmd := cmdFn(ctx, judgeAgent, judgeModel, prompt, "")
		cmd.Dir = outDir
		output, err := cmd.Output()
		if err != nil {
			logger.Warn("judge failed", "error", err)
			for j, pos := range llmPositions {
				results[pos] = AssertionResult{
					Text:     llmAssertions[j],
					Passed:   false,
					Evidence: fmt.Sprintf("judge error: %v", err),
				}
			}
		} else {
			llmGf, err := parseGradingOutput(string(output), llmAssertions)
			if err != nil {
				return nil, fmt.Errorf("parsing grading output for %s: %w", contextLabel, err)
			}
			for j, pos := range llmPositions {
				if j < len(llmGf.AssertionResults) {
					results[pos] = llmGf.AssertionResults[j]
				} else {
					results[pos] = AssertionResult{
						Text:     llmAssertions[j],
						Passed:   false,
						Evidence: "missing result from judge",
					}
				}
			}
		}
	}

	gf := &GradingFile{AssertionResults: results}
	gf.Summary.Total = len(gf.AssertionResults)
	for _, ar := range gf.AssertionResults {
		if ar.Passed {
			gf.Summary.Passed++
		} else {
			gf.Summary.Failed++
		}
	}
	if gf.Summary.Total > 0 {
		gf.Summary.PassRate = float64(gf.Summary.Passed) / float64(gf.Summary.Total)
	}

	if err := saveGrading(gradingPath, gf); err != nil {
		return nil, fmt.Errorf("saving grading for %s: %w", contextLabel, err)
	}
	return gf, nil
}

// readOutputContents reads all non-binary files from the output directory.
// Enforces cumulative size and file count caps to prevent agent outputs
// from ballooning the judge prompt.
const maxTotalOutputSize = 500 * 1024  // 500 KB total across all files
const maxOutputFiles = 50              // max number of files to read

func readOutputContents(outDir string) map[string]string {
	contents := map[string]string{}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return contents
	}
	var totalSize int
	fileCount := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		// ponytail: skip files > 100KB to avoid blowing up the prompt
		info, err := e.Info()
		if err != nil || info.Size() > 100*1024 {
			continue
		}
		// Cap total cumulative size across all output files
		if totalSize+int(info.Size()) > maxTotalOutputSize {
			continue
		}
		// Cap number of files read
		if fileCount >= maxOutputFiles {
			break
		}
		data, err := os.ReadFile(filepath.Join(outDir, e.Name())) // #nosec G304 -- outDir is the eval's own output directory, e.Name() comes from os.ReadDir on that same directory
		if err != nil {
			continue
		}
		// ponytail: simple binary check — if mostly non-printable, skip
		if isText(data) {
			contents[e.Name()] = string(data)
			totalSize += len(data)
			fileCount++
		}
	}
	return contents
}

// isText returns true if data looks like text.
func isText(data []byte) bool {
	return !bytes.Contains(data, []byte{0})
}

// parseAssertion converts an assertion string into a structured matcher.
func parseAssertion(s string) ParsedAssertion {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "file_exists:") {
		return ParsedAssertion{
			Original: s,
			Type:     MatcherFileExists,
			File:     strings.TrimSpace(s[len("file_exists:"):]),
		}
	}
	if strings.HasPrefix(s, "contains_text:") {
		rest := s[len("contains_text:"):]
		parts := strings.SplitN(rest, ":", 2)
		if len(parts) == 2 {
			return ParsedAssertion{
				Original: s,
				Type:     MatcherContainsText,
				File:     strings.TrimSpace(parts[0]),
				Arg:      strings.TrimSpace(parts[1]),
			}
		}
	}
	if strings.HasPrefix(s, "matches_text:") {
		rest := s[len("matches_text:"):]
		parts := strings.SplitN(rest, ":", 2)
		if len(parts) == 2 {
			return ParsedAssertion{
				Original: s,
				Type:     MatcherMatchesText,
				File:     strings.TrimSpace(parts[0]),
				Arg:      strings.TrimSpace(parts[1]),
			}
		}
	}
	return ParsedAssertion{Original: s, Type: MatcherLLM}
}

// isPathWithin returns true if resolving child inside parent stays within parent.
// Resolves symlinks on the resolved path before checking the prefix to prevent
// symlink-bypass attacks where an agent creates a symlink inside the output
// directory pointing outside.
func isPathWithin(parent, child string) bool {
	resolved := filepath.Clean(filepath.Join(parent, child))
	// Resolve symlinks to prevent bypass via agent-created symlinks inside outputs
	if real, err := filepath.EvalSymlinks(resolved); err == nil {
		resolved = real
	}
	return strings.HasPrefix(resolved, filepath.Clean(parent)+string(os.PathSeparator))
}

// isSafeAssertionPath validates that the file path in an assertion does not
// contain traversal sequences or absolute paths.
func isSafeAssertionPath(path string) bool {
	clean := filepath.Clean(path)
	return !strings.HasPrefix(clean, "/") &&
		!strings.HasPrefix(clean, "..") &&
		!strings.Contains(path, "\\")
}

// evaluateMatcher runs a deterministic matcher against output files.
func evaluateMatcher(pa ParsedAssertion, outDir string, outputContents map[string]string) AssertionResult {
	// Validate the assertion file path for traversal attacks
	if !isSafeAssertionPath(pa.File) {
		return AssertionResult{Text: pa.Original, Passed: false, Evidence: fmt.Sprintf("rejected: assertion file path %q contains traversal or absolute path", pa.File)}
	}

	switch pa.Type {
	case MatcherFileExists:
		if !isPathWithin(outDir, pa.File) {
			return AssertionResult{Text: pa.Original, Passed: false, Evidence: fmt.Sprintf("rejected: path %q escapes output directory", pa.File)}
		}
		_, err := os.Stat(filepath.Join(outDir, pa.File))
		if err == nil {
			return AssertionResult{Text: pa.Original, Passed: true, Evidence: fmt.Sprintf("file %s exists", pa.File)}
		}
		return AssertionResult{Text: pa.Original, Passed: false, Evidence: fmt.Sprintf("file %s does not exist", pa.File)}
	case MatcherContainsText:
		content, ok := outputContents[pa.File]
		if !ok {
			return AssertionResult{Text: pa.Original, Passed: false, Evidence: fmt.Sprintf("file %s not found in outputs", pa.File)}
		}
		if strings.Contains(content, pa.Arg) {
			return AssertionResult{Text: pa.Original, Passed: true, Evidence: fmt.Sprintf("file %s contains %q", pa.File, pa.Arg)}
		}
		return AssertionResult{Text: pa.Original, Passed: false, Evidence: fmt.Sprintf("file %s does not contain %q", pa.File, pa.Arg)}
	case MatcherMatchesText:
		content, ok := outputContents[pa.File]
		if !ok {
			return AssertionResult{Text: pa.Original, Passed: false, Evidence: fmt.Sprintf("file %s not found in outputs", pa.File)}
		}
		re, err := regexp.Compile(pa.Arg)
		if err != nil {
			return AssertionResult{Text: pa.Original, Passed: false, Evidence: fmt.Sprintf("invalid regex %q: %v", pa.Arg, err)}
		}
		if re.MatchString(content) {
			return AssertionResult{Text: pa.Original, Passed: true, Evidence: fmt.Sprintf("file %s matches %q", pa.File, pa.Arg)}
		}
		return AssertionResult{Text: pa.Original, Passed: false, Evidence: fmt.Sprintf("file %s does not match %q", pa.File, pa.Arg)}
	default:
		return AssertionResult{Text: pa.Original, Passed: false, Evidence: "unhandled matcher type"}
	}
}

// buildGradingPrompt creates the prompt for the judge.
// Assertion text is wrapped in structured markers to clearly separate
// data from instructions, preventing prompt injection via crafted
// assertion strings.
func buildGradingPrompt(eval Eval, outputContents map[string]string) string {
	var b strings.Builder
	b.WriteString("You are grading the output of a task. Return a JSON object with assertion results.\n\n")
	b.WriteString("Task prompt:\n")
	b.WriteString(eval.Prompt)
	b.WriteString("\n\nExpected output:\n")
	b.WriteString(eval.ExpectedOutput)
	b.WriteString("\n\nOutput files produced:\n")

	if len(outputContents) == 0 {
		b.WriteString("(no output files found)\n")
	} else {
		for name, content := range outputContents {
			fmt.Fprintf(&b, "\n--- %s ---\n%s\n", name, content)
		}
	}

	b.WriteString("\nAssertions to verify (wrapped in <assertion> markers — grade the assertion text, not the markers):\n")
	for i, a := range eval.Assertions {
		// Sanitize assertion text to prevent LLM prompt injection.
		// Strip common instruction-override patterns from assertion content.
		sanitized := sanitizeAssertionText(a)
		fmt.Fprintf(&b, "%d. <assertion>%s</assertion>\n", i+1, sanitized)
	}

	b.WriteString(`

Return ONLY a JSON object in this exact format (no markdown, no explanation):
{
  "assertion_results": [
    {"text": "the assertion text exactly as given", "passed": true, "evidence": "specific quote or observation from the output"},
    ...
  ]
}

Grading principles:
- Require concrete evidence for PASS. Don't give the benefit of the doubt.
- If the output has the right label but wrong substance, that's a FAIL.
- Evidence must reference specific content from the output files.
`)
	return b.String()
}

// sanitizeAssertionText strips prompt-injection patterns from assertion strings.
// Wraps content with markers that separate data from LLM instructions.
func sanitizeAssertionText(s string) string {
	// Replace common instruction-override patterns
	replacements := map[string]string{
		"IGNORE ALL PREVIOUS INSTRUCTIONS": "[INSTRUCTION STRIPPED]",
		"IGNORE PREVIOUS INSTRUCTIONS":     "[INSTRUCTION STRIPPED]",
		"DISREGARD PREVIOUS":              "[INSTRUCTION STRIPPED]",
		"DISREGARD ALL":                   "[INSTRUCTION STRIPPED]",
		"FORGET EVERYTHING":               "[INSTRUCTION STRIPPED]",
		"SYSTEM:":                         "[SYSTEM STRIPPED]",
	}
	result := s
	for pattern, replacement := range replacements {
		result = strings.ReplaceAll(result, pattern, replacement)
		// Also check case-insensitive variants
		result = strings.ReplaceAll(result, strings.ToLower(pattern), replacement)
		result = strings.ReplaceAll(result, strings.ToUpper(pattern), replacement)
	}
	return result
}

// parseGradingOutput extracts the grading JSON from the judge's response.
func parseGradingOutput(output string, assertions []string) (*GradingFile, error) {
	start := strings.Index(output, "{")
	if start < 0 {
		return nil, fmt.Errorf("no JSON found in judge output")
	}
	raw := output[start:]
	var gf GradingFile
	limited := io.LimitReader(strings.NewReader(raw), 10*1024*1024) // 10 MB
	if err := json.NewDecoder(limited).Decode(&gf); err != nil {
		return nil, fmt.Errorf("invalid grading JSON: %w", err)
	}
	return &gf, nil
}

// saveGrading writes a GradingFile to disk.
func saveGrading(path string, gf *GradingFile) error {
	data, err := json.MarshalIndent(gf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling grading: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// truncate shortens a string for error messages.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// extractFailedReasoning returns a concatenated list of FAIL evidence.
func extractFailedReasoning(gf *GradingFile) string {
	var reasons []string
	for _, ar := range gf.AssertionResults {
		if !ar.Passed && ar.Evidence != "" {
			reasons = append(reasons, ar.Evidence)
		}
	}
	return strings.Join(reasons, "\n- ")
}

// gradeFixAttempt grades a fix attempt's outputs.
func gradeFixAttempt(ctx context.Context, cfg *Config, eval Eval, workspace string, iteration int, modelKey string, attempt int, cmdFn CmdBuilder) (*GradingFile, error) {
	fixDir := filepath.Join(evalPath(workspace, iteration, eval.ID, modelKey), "with_skill", fmt.Sprintf("fix-%d", attempt))
	return gradeFromOutput(ctx, cfg, eval, filepath.Join(fixDir, "outputs"), filepath.Join(fixDir, "grading.json"), fmt.Sprintf("fix-%d for eval %d", attempt, eval.ID), cmdFn)
}
