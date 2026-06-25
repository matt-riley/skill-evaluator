package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// gradeEval shells out to the judge agent to grade assertions against outputs.
func gradeEval(ctx context.Context, cfg *Config, eval Eval, workspace string, iteration int, configLabel string) (*GradingFile, error) {
	evalDir := evalPath(workspace, iteration, eval.ID)
	outDir := filepath.Join(evalDir, configLabel, "outputs")
	gradingPath := filepath.Join(evalDir, configLabel, "grading.json")

	if len(eval.Assertions) == 0 {
		return nil, fmt.Errorf("eval %d has no assertions to grade", eval.ID)
	}

	// Read output files to include in the grading prompt
	outputContents := readOutputContents(outDir)

	// Build grading prompt
	prompt := buildGradingPrompt(eval, outputContents)

	// Shell out to judge
	judgeAgent := cfg.Judge.Agent
	if judgeAgent == "" {
		judgeAgent = cfg.Defaults.Agent
	}
	judgeModel := cfg.Judge.Model
	if judgeModel == "" {
		judgeModel = cfg.Defaults.Model
	}

	cmd := buildAgentCmd(judgeAgent, judgeModel, prompt, "")
	cmd.Dir = outDir
	output, err := cmd.Output()
	if err != nil {
		// If judge fails, produce a failed grading
		gf := &GradingFile{
			Summary: GradingSummary{Total: len(eval.Assertions), Failed: len(eval.Assertions)},
		}
		for _, a := range eval.Assertions {
			gf.AssertionResults = append(gf.AssertionResults, AssertionResult{
				Text:     a,
				Passed:   false,
				Evidence: fmt.Sprintf("judge error: %v", err),
			})
		}
		saveGrading(gradingPath, gf)
		return gf, nil
	}

	gf, err := parseGradingOutput(string(output), eval.Assertions)
	if err != nil {
		return nil, fmt.Errorf("parsing grading output for eval %d (%s): %w", eval.ID, configLabel, err)
	}

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

	saveGrading(gradingPath, gf)
	return gf, nil
}

// readOutputContents reads all non-binary files from the output directory.
func readOutputContents(outDir string) map[string]string {
	contents := map[string]string{}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return contents
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		// ponytail: skip files > 100KB to avoid blowing up the prompt
		info, err := e.Info()
		if err != nil || info.Size() > 100*1024 {
			continue
		}
		data, err := os.ReadFile(filepath.Join(outDir, e.Name()))
		if err != nil {
			continue
		}
		// ponytail: simple binary check — if mostly non-printable, skip
		if isText(data) {
			contents[e.Name()] = string(data)
		}
	}
	return contents
}

// isText returns true if data looks like text.
func isText(data []byte) bool {
	return !bytes.Contains(data, []byte{0})
}

// buildGradingPrompt creates the prompt for the judge.
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

	b.WriteString("\nAssertions to verify:\n")
	for i, a := range eval.Assertions {
		fmt.Fprintf(&b, "%d. %s\n", i+1, a)
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

// parseGradingOutput extracts the grading JSON from the judge's response.
func parseGradingOutput(output string, assertions []string) (*GradingFile, error) {
	start := strings.Index(output, "{")
	if start < 0 {
		return nil, fmt.Errorf("no JSON found in judge output")
	}
	raw := output[start:]
	var gf GradingFile
	if err := json.NewDecoder(strings.NewReader(raw)).Decode(&gf); err != nil {
		return nil, fmt.Errorf("invalid grading JSON: %w\nraw: %s", err, truncate(raw, 500))
	}
	return &gf, nil
}

// saveGrading writes a GradingFile to disk.
func saveGrading(path string, gf *GradingFile) {
	data, _ := json.MarshalIndent(gf, "", "  ")
	_ = os.WriteFile(path, data, 0o644)
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
func gradeFixAttempt(ctx context.Context, cfg *Config, eval Eval, workspace string, iteration int, attempt int) (*GradingFile, error) {
	evalDir := evalPath(workspace, iteration, eval.ID)
	fixDir := filepath.Join(evalDir, "with_skill", fmt.Sprintf("fix-%d", attempt))
	outDir := filepath.Join(fixDir, "outputs")

	outputContents := readOutputContents(outDir)
	prompt := buildGradingPrompt(eval, outputContents)

	judgeAgent := cfg.Judge.Agent
	if judgeAgent == "" {
		judgeAgent = cfg.Defaults.Agent
	}
	judgeModel := cfg.Judge.Model
	if judgeModel == "" {
		judgeModel = cfg.Defaults.Model
	}

	cmd := buildAgentCmd(judgeAgent, judgeModel, prompt, "")
	cmd.Dir = outDir
	output, err := cmd.Output()
	if err != nil {
		gf := &GradingFile{
			Summary: GradingSummary{Total: len(eval.Assertions), Failed: len(eval.Assertions)},
		}
		for _, a := range eval.Assertions {
			gf.AssertionResults = append(gf.AssertionResults, AssertionResult{
				Text:     a,
				Passed:   false,
				Evidence: fmt.Sprintf("judge error: %v", err),
			})
		}
		saveGrading(filepath.Join(fixDir, "grading.json"), gf)
		return gf, nil
	}

	gf, err := parseGradingOutput(string(output), eval.Assertions)
	if err != nil {
		return nil, fmt.Errorf("parsing fix-%d grading for eval %d: %w", attempt, eval.ID, err)
	}

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

	saveGrading(filepath.Join(fixDir, "grading.json"), gf)
	return gf, nil
}
