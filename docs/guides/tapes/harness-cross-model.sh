#!/bin/bash
# Harness: simulates skill-eval loop --models for the cross-model VHS tape.

case "$1" in
  loop)
    shift
    models=()
    while [ $# -gt 0 ]; do
      case "$1" in
        --models) IFS=',' read -ra models <<< "$2"; shift 2 ;;
        --fix) shift ;;
        --max-fix-attempts) shift 2 ;;
        --baseline) shift 2 ;;
        *) shift ;;
      esac
    done

    echo "=== skill-eval loop ==="
    echo ""
    echo "[1/3] Running evals..."
    echo "Iteration 1 — 2 evals × ${#models[@]} model(s)"
    echo ""
    echo "Eval 1: Analyze sales data and chart top 3 months"
    for m in "${models[@]}"; do
      echo "  $m/with_skill     ok (18300ms)"
    done
    for m in "${models[@]}"; do
      echo "  $m/baseline       ok (12100ms)"
    done
    echo ""
    echo "Eval 2: Generate a summary report"
    for m in "${models[@]}"; do
      echo "  $m/with_skill     ok (9500ms)"
    done
    for m in "${models[@]}"; do
      echo "  $m/baseline       ok (8700ms)"
    done
    echo ""
    echo "Done."
    echo ""
    echo "[2/3] Grading..."
    echo "Grading iteration 1"

    # Simulate per-model results
    for m in "${models[@]}"; do
      if [ "$m" = "claude:opus-4-8" ]; then
        echo "  eval 1 $m/with_skill... 2/3 passed"
        echo "  eval 1 $m/baseline... 1/3 passed"
        echo "  eval 2 $m/with_skill... 1/2 passed"
        echo "  eval 2 $m/baseline... 2/2 passed"
      else
        echo "  eval 1 $m/with_skill... 3/3 passed"
        echo "  eval 1 $m/baseline... 1/3 passed"
        echo "  eval 2 $m/with_skill... 2/2 passed"
        echo "  eval 2 $m/baseline... 2/2 passed"
      fi
    done

    echo ""
    echo "[3/3] Benchmarking..."
    sleep 0.5
    echo "  models:"
    echo "    pi-claude-sonnet:   +33% (best)"
    echo "    claude-opus-4-8:    +17% (worst)"
    echo "    copilot-gpt-5:      +33%"
    echo ""
    echo "Loop complete."
    ;;
esac
