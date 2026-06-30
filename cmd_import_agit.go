package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/matt-riley/skill-evaluator/internal/agit"
)

// cmd_import_agit.go — the `skill-eval import-agit` CLI subcommand: flag
// parsing, evals.json output writing, and merge logic. The agit shell-out
// and conversion logic itself lives in internal/agit.

// evalFromConverted maps an agit.ConvertedEval (decoupled from package main)
// onto this package's own Eval/EvalSource schema.
func evalFromConverted(ce agit.ConvertedEval) Eval {
	return Eval{
		ID:             ce.ID,
		Prompt:         ce.Prompt,
		ExpectedOutput: ce.ExpectedOutput,
		Assertions:     ce.Assertions,
		Source: &EvalSource{
			AgitOrigin:     ce.Source.Origin,
			AgitSessionID:  ce.Source.SessionID,
			AgitStepHash:   ce.Source.StepHash,
			Timestamp:      ce.Source.Timestamp,
			EvalHash:       ce.Source.EvalHash,
			QualityScore:   ce.Source.QualityScore,
			Classification: ce.Source.Classification,
		},
	}
}

func cmdImportAgit(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("import-agit", flag.ContinueOnError)
	session := fs.String("session", "", "Specific agit session (origin/id); default: most recent")
	skillDir := fs.String("skill", "", "Skill directory to write evals.json into (default: detect upward)")
	outPath := fs.String("out", "", "Output path (default: <skill>/evals/evals.json)")
	force := fs.Bool("force", false, "Overwrite an existing evals.json")
	merge := fs.Bool("merge", false, "Merge into existing evals.json instead of overwriting")
	allSessions := fs.Bool("all-sessions", false, "Import all recorded agit sessions")
	evalFilterRaw := fs.String("eval-filter", "", "Filter sessions by agit eval classification (good,mixed,bad,unknown)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Parse eval filter into a set for O(1) lookup.
	evalFilter := agit.ParseEvalFilter(*evalFilterRaw)

	dir := *skillDir
	if dir == "" {
		d, err := detectSkillDir()
		if err != nil {
			return fmt.Errorf("no SKILL.md found — pass --skill <dir> or run from a skill directory: %w", err)
		}
		dir = d
	}

	// Determine the output path early so we can append across sessions.
	if *outPath == "" {
		*outPath = filepath.Join(dir, "evals", "evals.json")
	}

	// Collect all session targets.
	type sessionTarget struct {
		origin    string
		sessionID string
	}
	var targets []sessionTarget

	if *allSessions {
		sessions, err := agit.FetchSessions()
		if err != nil {
			return fmt.Errorf("listing sessions: %w (is agit installed and in PATH?)", err)
		}
		if len(sessions.Sessions) == 0 {
			return fmt.Errorf("no sessions recorded — record some agent activity first")
		}
		for _, s := range sessions.Sessions {
			targets = append(targets, sessionTarget{
				origin:    s.Origin,
				sessionID: s.SessionID,
			})
		}
		fmt.Printf("Found %d session(s)\n", len(targets))
	} else {
		targets = append(targets, sessionTarget{origin: "", sessionID: *session})
	}

	var allEvals []Eval
	for _, tgt := range targets {
		targetRef := tgt.sessionID
		if tgt.origin != "" {
			targetRef = fmt.Sprintf("%s/%s", tgt.origin, tgt.sessionID)
		}

		// Try agit steps --json first (agit v1.26+).
		steps, stepsErr := agit.FetchSteps(targetRef)
		if stepsErr == nil && steps != nil && len(steps.Steps) > 0 {
			// New fast path: single call, no N+1.
			fmt.Printf("Importing %s/%s — %d steps (via agit steps)\n", steps.Origin, steps.SessionID, len(steps.Steps))

			// Optionally run agit eval for quality metadata.
			var ae *agit.EvalReport
			if evalTargetRef := targetRef; evalTargetRef != "" {
				ae, _ = agit.FetchEvalReport(evalTargetRef) // best-effort; eval may not exist
			}

			converted := agit.ConvertSteps(steps, ae, evalFilter)
			for _, ce := range converted {
				allEvals = append(allEvals, evalFromConverted(ce))
			}
			continue
		}

		// Fallback: legacy log+show+diff pattern for older agit.
		if stepsErr != nil {
			logger.Info("agit steps not available, falling back to log+show+diff",
				"error", stepsErr,
				"session", targetRef,
			)
		}

		log, err := agit.FetchLog(targetRef)
		if err != nil {
			if len(targets) > 1 {
				logger.Warn("skipping session (log failed)", "session", targetRef, "error", err)
				continue
			}
			return err
		}
		if len(log.Steps) == 0 {
			if len(targets) > 1 {
				logger.Warn("skipping session (no steps)", "session", targetRef)
				continue
			}
			return fmt.Errorf("no steps recorded for session %s/%s", log.Origin, log.SessionID)
		}
		fmt.Printf("Importing %s/%s — %d steps (via log+show+diff)\n", log.Origin, log.SessionID, len(log.Steps))

		// Try eval for quality metadata on the legacy path too.
		var ae *agit.EvalReport
		if targetRef != "" {
			ae, _ = agit.FetchEvalReport(targetRef)
		}

		stepsByHash := make(map[string]agit.Step, len(log.Steps))
		diffsByHash := make(map[string]*agit.Diff, len(log.Steps))
		for _, row := range log.Steps {
			show, err := agit.FetchShow(row.Hash)
			if err != nil {
				logger.Warn("skipping step (show failed)", "hash", row.Hash, "error", err)
				continue
			}
			stepsByHash[row.Hash] = show.Step
			diff, err := agit.FetchDiff(row.Hash)
			if err != nil {
				logger.Warn("no diff for step", "hash", row.Hash, "error", err)
				diff = &agit.Diff{}
			}
			diffsByHash[row.Hash] = diff
		}

		classification := agit.EvalClassification(ae)
		if len(evalFilter) > 0 && !evalFilter[classification] {
			logger.Info("skipping session (eval filter)",
				"origin", log.Origin,
				"session_id", log.SessionID,
				"classification", classification,
			)
			continue
		}

		converted := agit.ConvertSession(stepsByHash, diffsByHash, log.Steps, log.Origin, log.SessionID)
		for _, ce := range converted {
			// Enrich with eval metadata if available.
			if ae != nil {
				ce.Source.EvalHash = ae.EvalHash
				ce.Source.Classification = agit.EvalClassification(ae)
				if ae.InScopeAssessment.Dimensions != nil {
					ce.Source.QualityScore = agit.EvalQualityScore(ae.InScopeAssessment.Dimensions)
				}
			}
			allEvals = append(allEvals, evalFromConverted(ce))
		}
	}

	if len(allEvals) == 0 {
		return fmt.Errorf("no task-like turns found (all user prompts shorter than %d chars, filtered, or no file changes)",
			agit.MinPromptLen)
	}

	evalFile := EvalFile{
		SkillName: filepath.Base(dir),
		Evals:     allEvals,
	}

	if *merge {
		existing, err := readEvalsFile(*outPath)
		if err == nil {
			// Append new evals after existing ones, renumbering.
			nextID := 1
			for _, e := range existing.Evals {
				if e.ID >= nextID {
					nextID = e.ID + 1
				}
			}
			for i := range evalFile.Evals {
				evalFile.Evals[i].ID = nextID + i
			}
			evalFile.Evals = append(existing.Evals, evalFile.Evals...)
			fmt.Printf("Merging with %d existing evals (next ID: %d)\n", len(existing.Evals), nextID)
		}
	} else if _, err := os.Stat(*outPath); err == nil && !*force {
		return fmt.Errorf("%s already exists — pass --force to overwrite, --merge to append, or --out <path>", *outPath)
	}

	if err := os.MkdirAll(filepath.Dir(*outPath), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(evalFile, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(*outPath, data, 0o600); err != nil {
		return err
	}
	fmt.Printf("Wrote %d evals to %s\n", len(evalFile.Evals), *outPath)
	return nil
}

// readEvalsFile reads an existing evals.json with size validation.
// Used only for the merge path where the existing file must also pass
// reasonable size checks.
func readEvalsFile(path string) (*EvalFile, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	const maxMergeFileSize = 2 * 1024 * 1024 // 2 MB for merge input
	if fi.Size() > maxMergeFileSize {
		return nil, fmt.Errorf("%s is too large for merge: %d bytes (max %d)", path, fi.Size(), maxMergeFileSize)
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is the resolved evals.json output path from --out/--skill CLI flags or the default skill-dir convention
	if err != nil {
		return nil, err
	}
	var ef EvalFile
	if err := json.Unmarshal(data, &ef); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &ef, nil
}
