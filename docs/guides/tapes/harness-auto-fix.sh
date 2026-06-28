#!/bin/bash
# Harness: simulates skill-eval loop --fix for the auto-fixing VHS tape.
# Shows an eval failing initially, then improving through fix attempts.

case "$1" in
  loop)
    shift
    fix_mode=""
    max_attempts="3"
    while [ $# -gt 0 ]; do
      case "$1" in
        --fix) fix_mode=1 ;;
        --max-fix-attempts) max_attempts="$2"; shift ;;
        --baseline) shift ;;
      esac
      shift
    done

    echo "=== skill-eval loop ==="
    echo ""
    echo "[1/3] Running evals..."
    echo "Iteration 1 — 2 evals"
    echo ""
    echo "Eval 1: Analyze sales data and chart top 3 months"
    echo "  with_skill... ok (18300ms)"
    echo "  baseline... ok (12100ms)"
    echo ""
    echo "Eval 2: Generate a summary report"
    echo "  with_skill... ok (9500ms)"
    echo "  baseline... ok (8700ms)"
    echo ""
    echo "Done."
    echo ""
    echo "[2/3] Grading..."
    echo "Grading iteration 1"
    echo "  eval 1 with_skill... 3/3 passed"
    echo "  eval 1 baseline... 1/3 passed"
    sleep 0.5
    echo "  eval 2 with_skill... 1/2 passed"
    echo "  eval 2 baseline... 2/2 passed"

    if [ -n "$fix_mode" ]; then
      echo ""
      echo "[3/4] Auto-fixing failed evals..."
      sleep 0.6
      echo "  eval 1: already passing, skipping"
      sleep 0.3
      echo "  eval 2: 1/2 failed — fixing..."
      sleep 0.8
      echo "    attempt 1: 1/2 passed"
      sleep 0.5
      echo "    attempt 2: 2/2 passed ✓"
      sleep 0.3
      echo "    best: attempt 2 (100% pass)"
      echo ""
      echo "[4/4] Benchmarking..."
      sleep 0.4
    else
      echo ""
      echo "[3/3] Benchmarking..."
      sleep 0.4
    fi

    echo "  ✓ benchmark.json written"
    echo ""
    echo "Loop complete."
    ;;
esac
