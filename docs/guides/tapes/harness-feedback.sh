#!/bin/bash
# Simulates skill-eval grade for feedback-loop VHS tape.
case "$1" in
  grade)
    echo "▶ Re-running grade with feedback context..."
    sleep 0.8
    if [ -f workspace/iteration-1/feedback.json ]; then
      echo "  ✓ grading.json updated"
    fi
    ;;
esac
