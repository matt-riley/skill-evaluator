#!/usr/bin/env bash
# Simulated output for the VHS demo. Matches the shape of `skill-eval loop`.
set -euo pipefail

printf '\033[1m=== skill-eval loop ===\033[0m\n'

printf '\n\033[1m[1/3] Running evals...\033[0m\n'
sleep 0.6
printf 'Iteration 2 — 2 evals\n'
sleep 0.3
printf '\nEval 1: Build a bar chart of quarterly sales\n'
sleep 0.2
printf '  with_skill... ok (1.8s)\n'
sleep 0.2
printf '  baseline... ok (2.4s)\n'
sleep 0.2
printf '\nEval 2: Label both axes on the chart\n'
sleep 0.2
printf '  with_skill... ok (2.1s)\n'
sleep 0.2
printf '  baseline... ok (3.2s)\n'
sleep 0.2

printf '\n\033[1m[2/3] Grading...\033[0m\n'
sleep 0.5
printf 'Grading iteration 2\n'
sleep 0.2
printf '  eval 1 with_skill... 4/4 passed\n'
sleep 0.2
printf '  eval 1 baseline... 3/4 passed\n'
sleep 0.2
printf '  eval 2 with_skill... 3/3 passed\n'
sleep 0.2
printf '  eval 2 baseline... 1/3 passed\n'
sleep 0.5

printf '\nBenchmark saved to benchmark.json\n'
printf '  Pass rate: +25.0%% with skill\n'
printf '  Avg time:  -0.8s with skill\n'
printf '  Avg tokens: -180 with skill\n'

printf '\n\033[1m[3/3] Loop complete.\033[0m\n'
sleep 1.5
