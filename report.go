package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"time"
)

// ReportData is the template payload for the HTML report.
type ReportData struct {
	SkillName         string
	Iteration         int
	GeneratedAt       time.Time
	PreviousIteration int
	IterationDelta    *Delta
	Models            map[string]ModelBenchmark
	BestModel         string
	WorstModel        string
	Suggestions       []string
	LLMSuggestions    string
}

const reportTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Benchmark Report — {{.SkillName}} Iteration {{.Iteration}}</title>
  <style>
    body { font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; max-width: 920px; margin: 2rem auto; padding: 0 1rem; color: #1f2328; line-height: 1.5; }
    h1, h2 { border-bottom: 1px solid #d0d7de; padding-bottom: .3rem; }
    table { border-collapse: collapse; width: 100%; margin: 1rem 0; }
    th, td { text-align: left; padding: .5rem .75rem; border-bottom: 1px solid #eef1f4; }
    th { font-weight: 600; color: #57606a; }
    .good { color: #1a7f37; }
    .bad { color: #d1242f; }
    .neutral { color: #57606a; }
    .summary { background: #f6f8fa; padding: 1rem; border-radius: 6px; margin: 1rem 0; }
    .suggestions { margin: 1rem 0; padding-left: 1.25rem; }
    .suggestions li { margin: .5rem 0; }
    .llm-notes { background: #f6f8fa; padding: 1rem; border-radius: 6px; white-space: pre-wrap; }
    small { color: #57606a; }
  </style>
</head>
<body>
  <h1>Benchmark Report — {{.SkillName}}</h1>
  <p class="neutral">Iteration {{.Iteration}} · Generated {{.GeneratedAt.Format "2006-01-02 15:04:05"}}</p>

  {{if .IterationDelta}}
  <div class="summary">
    <h2>Trend vs Iteration {{.PreviousIteration}}</h2>
    <p>Pass-rate change: <strong class="{{passRateClass .IterationDelta.PassRate}}">{{printf "%+.1f%%" (percent .IterationDelta.PassRate)}}</strong></p>
    <p>Time change: <strong class="{{costClass .IterationDelta.TimeSeconds}}">{{printf "%+.1fs" .IterationDelta.TimeSeconds}}</strong></p>
    <p>Token change: <strong class="{{costClass .IterationDelta.Tokens}}">{{printf "%+.0f" .IterationDelta.Tokens}}</strong></p>
  </div>
  {{end}}

  <h2>Per-Model Results</h2>
  <table>
    <thead>
      <tr>
        <th>Model</th>
        <th>With-skill pass rate</th>
        <th>Baseline pass rate</th>
        <th>Pass-rate delta</th>
        <th>Time delta</th>
        <th>Token delta</th>
      </tr>
    </thead>
    <tbody>
      {{range $mk, $mb := .Models}}
      <tr>
        <td>
          {{$mk}}
          {{if eq $mk $.BestModel}}<span title="Best performer">🏆</span>{{end}}
          {{if eq $mk $.WorstModel}}<span title="Worst performer">⚠️</span>{{end}}
        </td>
        <td>{{printf "%.1f%%" (percent $mb.WithSkill.PassRate.Mean)}}</td>
        <td>{{printf "%.1f%%" (percent $mb.Baseline.PassRate.Mean)}}</td>
        <td class="{{passRateClass $mb.Delta.PassRate}}">{{printf "%+.1f%%" (percent $mb.Delta.PassRate)}}</td>
        <td class="{{costClass $mb.Delta.TimeSeconds}}">{{printf "%+.1fs" $mb.Delta.TimeSeconds}}</td>
        <td class="{{costClass $mb.Delta.Tokens}}">{{printf "%+.0f" $mb.Delta.Tokens}}</td>
      </tr>
      {{end}}
    </tbody>
  </table>

  <h2>Suggested Next Steps</h2>
  <ul class="suggestions">
    {{range .Suggestions}}
    <li>{{.}}</li>
    {{else}}
    <li>No specific suggestions — review the per-model table above.</li>
    {{end}}
  </ul>

  <p><small>Raw data: <code>benchmark.json</code> in the same iteration directory.</small></p>

  {{if .LLMSuggestions}}
  <h2>LLM Coach Notes</h2>
  {{/* SECURITY: LLMSuggestions comes from untrusted LLM output. html/template
       auto-escapes it, which prevents active XSS. NEVER wrap .LLMSuggestions
       or .SkillName in template.HTML() — doing so would disable escaping and
       create a stored XSS vulnerability. */}}
  <div class="llm-notes">{{.LLMSuggestions}}</div>
  {{end}}
</body>
</html>`

// cmdReport generates an HTML report from a benchmark.json file.
func cmdReport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	iterFlag := fs.Int("iteration", 0, "Target iteration (default: latest)")
	llmFlag := fs.Bool("llm-suggestions", false, "Ask the judge agent for additional suggestions")
	if err := fs.Parse(args); err != nil {
		return err
	}

	skillDir, err := detectSkillDir()
	if err != nil {
		return err
	}

	cfg, err := LoadConfig(skillDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	ef, err := readEvals(skillDir)
	if err != nil {
		return err
	}

	ws := workspacePath(skillDir)
	iter := nextIteration(ws) - 1
	if *iterFlag > 0 {
		iter = *iterFlag
	}
	if iter < 1 {
		return fmt.Errorf("no iterations found")
	}

	bf, err := loadBenchmarkFile(ws, iter)
	if err != nil {
		return fmt.Errorf("loading benchmark for iteration %d: %w", iter, err)
	}

	data := &ReportData{
		SkillName:         ef.SkillName,
		Iteration:         iter,
		GeneratedAt:       bf.GeneratedAt,
		PreviousIteration: bf.PreviousIteration,
		IterationDelta:    bf.IterationDelta,
		Models:            bf.Models,
		BestModel:         bf.BestModel,
		WorstModel:        bf.WorstModel,
	}
	data.Suggestions = buildSuggestions(data)

	if *llmFlag {
		notes, err := llmCoachNotes(context.Background(), cfg, data, bf, nil)
		if err != nil {
			data.Suggestions = append(data.Suggestions, fmt.Sprintf("LLM coach notes unavailable: %v", err))
		} else {
			data.LLMSuggestions = notes
		}
	}

	out, err := renderReport(data)
	if err != nil {
		return fmt.Errorf("rendering report: %w", err)
	}

	outPath := filepath.Join(iterationPath(ws, iter), "report.html")
	if err := os.WriteFile(outPath, out, 0o600); err != nil {
		return fmt.Errorf("writing report: %w", err)
	}

	fmt.Printf("Report written to %s\n", outPath)
	return nil
}

// loadBenchmarkFile reads a benchmark.json file for a specific iteration.
func loadBenchmarkFile(workspace string, iter int) (*BenchmarkFile, error) {
	path := filepath.Join(iterationPath(workspace, iter), "benchmark.json")
	data, err := os.ReadFile(path) // #nosec G304 -- path built via iterationPath(), internal workspace convention
	if err != nil {
		return nil, err
	}
	var bf BenchmarkFile
	if err := json.Unmarshal(data, &bf); err != nil {
		return nil, err
	}
	return &bf, nil
}

// llmCoachNotes asks the configured judge agent for extra coaching advice.
func llmCoachNotes(ctx context.Context, cfg *Config, data *ReportData, bf *BenchmarkFile, cmdFn CmdBuilder) (string, error) {
	payload, err := json.MarshalIndent(bf, "", "  ")
	if err != nil {
		return "", err
	}

	prompt := fmt.Sprintf(`You are reviewing a skill-evaluation benchmark for the skill %q, iteration %d.

The benchmark data below shows how models performed with and without the skill, plus deltas and the previous iteration.

Give concise, actionable advice for the skill author. Focus on:
1. Whether the evals look discriminative (does the skill actually help, or are they too easy?)
2. What the next iteration should test or change.
3. Any suspect patterns (e.g., high baseline, negative skill delta, missing token counts).

Keep it to 3-5 short paragraphs. Do not use markdown headings.

Benchmark JSON:
%s
`, data.SkillName, data.Iteration, payload)

	agent := cfg.Judge.Agent
	if agent == "" {
		agent = cfg.Defaults.Agent
	}
	model := cfg.Judge.Model
	if model == "" {
		model = cfg.Defaults.Model
	}

	if cmdFn == nil {
		cmdFn = buildAgentCmd
	}
	cmd := cmdFn(ctx, agent, model, prompt, "")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// renderReport executes the HTML template against the report data.
func renderReport(data *ReportData) ([]byte, error) {
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"percent":       percent,
		"passRateClass": passRateClass,
		"costClass":     costClass,
	}).Parse(reportTemplate)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// percent converts a ratio to a percentage.
func percent(v float64) float64 {
	return v * 100
}

// passRateClass labels a delta where higher is better.
func passRateClass(v float64) string {
	if v > 0 {
		return "good"
	}
	if v < 0 {
		return "bad"
	}
	return "neutral"
}

// costClass labels a delta where lower is better (time, tokens).
func costClass(v float64) string {
	if v < 0 {
		return "good"
	}
	if v > 0 {
		return "bad"
	}
	return "neutral"
}

// buildSuggestions creates actionable next-step advice from the benchmark data.
func buildSuggestions(data *ReportData) []string {
	if len(data.Models) == 0 {
		return []string{"No benchmark data was found. Check that runs finished and grading files exist."}
	}

	var suggestions []string

	if data.IterationDelta != nil && data.PreviousIteration > 0 {
		change := data.IterationDelta.PassRate
		switch {
		case change > 0:
			suggestions = append(suggestions, fmt.Sprintf("Pass rates improved by %.1f%% versus iteration %d — keep the recent changes.", change*100, data.PreviousIteration))
		case change < 0:
			suggestions = append(suggestions, fmt.Sprintf("Pass rates regressed by %.1f%% versus iteration %d — review the latest skill or eval changes.", -change*100, data.PreviousIteration))
		default:
			suggestions = append(suggestions, fmt.Sprintf("Pass rates are unchanged versus iteration %d — try a more targeted change.", data.PreviousIteration))
		}
	}

	if data.BestModel != "" && data.WorstModel != "" && data.BestModel != data.WorstModel {
		suggestions = append(suggestions, fmt.Sprintf("Best performer was %s; worst was %s. Compare their outputs to find what helps.", data.BestModel, data.WorstModel))
	}

	var sumWithSkillPass float64
	var modelCount int
	allTokensZero := true

	for mk, mb := range data.Models {
		sumWithSkillPass += mb.WithSkill.PassRate.Mean
		modelCount++

		if mb.WithSkill.Tokens.Mean > 0 || mb.Baseline.Tokens.Mean > 0 {
			allTokensZero = false
		}

		wsRate := mb.WithSkill.PassRate.Mean
		switch {
		case wsRate >= 0.95:
			suggestions = append(suggestions, fmt.Sprintf("%s is near-perfect with the skill (%.1f%%) — consider expanding eval coverage.", mk, wsRate*100))
		case wsRate < 0.5:
			suggestions = append(suggestions, fmt.Sprintf("%s scores below 50%% even with the skill — focus debugging there first.", mk))
		}

		if mb.Delta.PassRate < 0 {
			suggestions = append(suggestions, fmt.Sprintf("The skill currently hurts %s (%.1f%% drop) — try simplifying the prompt or tightening the assertions.", mk, -mb.Delta.PassRate*100))
		}

		baselineTime := mb.Baseline.TimeSeconds.Mean
		if baselineTime > 0 && mb.Delta.TimeSeconds/baselineTime > 0.2 {
			suggestions = append(suggestions, fmt.Sprintf("%s with-skill runs are >20%% slower than baseline — look for redundant instructions.", mk))
		}
	}

	if modelCount > 0 {
		avg := sumWithSkillPass / float64(modelCount)
		if avg > 0.9 {
			suggestions = append(suggestions, "Overall with-skill pass rate is very high — the skill is broadly effective.")
		} else if avg < 0.5 {
			suggestions = append(suggestions, "Overall with-skill pass rate is low — revisit the skill examples and core assertions before adding more evals.")
		}
	}

	// If the baseline already ace's the evals, the evals may not be isolating the skill's value.
	if modelCount > 0 {
		var sumBaselinePass float64
		var lowDeltaCount int
		for _, mb := range data.Models {
			sumBaselinePass += mb.Baseline.PassRate.Mean
			if mb.Delta.PassRate <= 0.05 {
				lowDeltaCount++
			}
		}
		avgBaseline := sumBaselinePass / float64(modelCount)
		if avgBaseline >= 0.95 && lowDeltaCount == modelCount {
			suggestions = append(suggestions, "Baseline already passes nearly all evals — they may be too easy or not isolating the skill's value. Add harder cases that require the skill's guidance, or check that assertions actually verify skill-specific behavior rather than generic correctness.")
		}
	}

	if allTokensZero {
		suggestions = append(suggestions, "Token counts are all zero — the token-extraction heuristic probably missed this agent’s output format.")
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "No strong trends detected. Review the per-model table to decide what to iterate on next.")
	}

	return suggestions
}
