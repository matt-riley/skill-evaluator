// Package agit bridges recorded agit sessions into skill-evaluator eval
// candidates. It shells out to `agit` (the same shell-out pattern used for
// agent runtimes) and turns each substantive user turn into a candidate
// whose ground-truth expected output and assertions are derived from the
// recorded outcome (git state, produced files, assistant summary).
//
// ponytail: agit is the source of truth for "what happened"; skill-evaluator
// owns the evals.json format. This package is the thin bridge between them.
package agit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
)

// Logger defaults to a no-op logger so callers that import this package
// without wiring up logging don't panic. main.go's initLogger sets this to
// the real logger after creating it (this package cannot import package
// main, since main imports this package).
var Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

// --- agit access (shell-out) ---

// runAgit is swappable in tests. Uses exec.LookPath to resolve the agit
// binary to a fixed path, preventing PATH-based binary hijacking.
var runAgit = func(args ...string) ([]byte, error) {
	agitPath, err := exec.LookPath("agit")
	if err != nil {
		return nil, fmt.Errorf("agit not found in PATH: %w", err)
	}
	// #nosec G204 -- path resolved via exec.LookPath, args are static subcommand strings defined in this package
	return exec.Command(agitPath, args...).Output()
}

func FetchLog(session string) (*Log, error) {
	args := []string{"log", "--json"}
	if session != "" {
		args = append(args, session)
	}
	out, err := runAgit(args...)
	if err != nil {
		return nil, fmt.Errorf("agit log: %w", err)
	}
	return decodeEnvelope[Log](out)
}

func FetchShow(hash string) (*Show, error) {
	out, err := runAgit("show", "--json", hash)
	if err != nil {
		return nil, fmt.Errorf("agit show %s: %w", hash, err)
	}
	return decodeEnvelope[Show](out)
}

func FetchDiff(hash string) (*Diff, error) {
	out, err := runAgit("diff", "--json", hash)
	if err != nil {
		return nil, fmt.Errorf("agit diff %s: %w", hash, err)
	}
	return decodeEnvelope[Diff](out)
}

// FetchSteps calls `agit steps --json --include-step-objects [session]`
// to fetch all steps with messages, tool_calls, and diffs in a single call.
// This replaces the N+1 log+show+diff pattern for agit v1.26+.
func FetchSteps(session string) (*Steps, error) {
	args := []string{"steps", "--json", "--include-step-objects"}
	if session != "" {
		args = append(args, session)
	}
	out, err := runAgit(args...)
	if err != nil {
		return nil, fmt.Errorf("agit steps: %w", err)
	}
	return decodeEnvelope[Steps](out)
}

// FetchSessions calls `agit sessions --json` to list all recorded sessions.
func FetchSessions() (*Sessions, error) {
	out, err := runAgit("sessions", "--json")
	if err != nil {
		return nil, fmt.Errorf("agit sessions: %w", err)
	}
	return decodeEnvelope[Sessions](out)
}

// FetchEvalReport calls `agit eval --json [origin/session]` to get the
// session classification and quality dimensions.
func FetchEvalReport(session string) (*EvalReport, error) {
	args := []string{"eval", "--json"}
	if session != "" {
		args = append(args, session)
	}
	out, err := runAgit(args...)
	if err != nil {
		return nil, fmt.Errorf("agit eval: %w", err)
	}
	return decodeEnvelope[EvalReport](out)
}

// decodeEnvelope unwraps agit's cli-json-v1 envelope.
func decodeEnvelope[T any](raw []byte) (*T, error) {
	var env struct {
		Data T `json:"data"`
	}
	limited := io.LimitReader(bytes.NewReader(raw), 50*1024*1024) // 50 MB for agit output
	if err := json.NewDecoder(limited).Decode(&env); err != nil {
		return nil, fmt.Errorf("decoding agit JSON: %w", err)
	}
	return &env.Data, nil
}
