Output docs/site/public/guides/reading-results.mp4

Set Shell "bash"
Set FontSize 18
Set Width 1400
Set Height 800
Set Theme "Catppuccin Mocha"
Set Padding 24
Set TypingSpeed 40ms

Hide
Type "WKSP=$(mktemp -d) && cd $WKSP && mkdir -p workspace/iteration-1"
Enter
Type "cat > workspace/iteration-1/benchmark.json << 'ENDJSON'"
Enter
Type `{`
Enter
Type `  "summary": {`
Enter
Type `    "total_evals": 3,`
Enter
Type `    "pass_rate_with_skill": 0.89,`
Enter
Type `    "pass_rate_baseline": 0.56,`
Enter
Type `    "delta": "+33%",`
Enter
Type `    "avg_duration_with_skill_s": 18.4,`
Enter
Type `    "avg_duration_baseline_s": 14.2`
Enter
Type `  },`
Enter
Type `  "evals": {`
Enter
Type `    "eval-1": { "with_skill": "FAIL", "baseline": "FAIL", "delta": "none" },`
Enter
Type `    "eval-2": { "with_skill": "PASS", "baseline": "PASS", "delta": "none" },`
Enter
Type `    "eval-3": { "with_skill": "PASS", "baseline": "FAIL", "delta": "+skill" }`
Enter
Type `  }`
Enter
Type `}`
Enter
Type "ENDJSON"
Enter
Show

Type "# Open up benchmark.json to see the big picture..."
Sleep 600ms
Enter
Sleep 300ms
Type "cat workspace/iteration-1/benchmark.json | jq .summary"
Enter
Sleep 1.5s

Type "# +33% pass rate with the skill vs baseline! 🚀"
Sleep 800ms
Enter
Sleep 400ms
Type "# Let's drill into per-eval results..."
Sleep 600ms
Enter
Sleep 300ms
Type "cat workspace/iteration-1/benchmark.json | jq '.evals | to_entries[] | {eval: .key, result: .value}'"
Enter
Sleep 1.8s

Type "# eval-3: FAIL → PASS — that's your skill making the difference ✨"
Sleep 1.8s
Enter
