Output docs/site/public/guides/first-eval.mp4

Set Shell "bash"
Set FontSize 18
Set Width 1400
Set Height 800
Set Theme "Catppuccin Mocha"
Set Padding 24
Set TypingSpeed 45ms

Hide
Type `ORIG=$(pwd) && skill-eval() { bash "$ORIG/docs/guides/tapes/harness-first-eval.sh" "$@"; }`
Enter
Type "SKILLDIR=$(mktemp -d) && cd $SKILLDIR"
Enter
Type "clear"
Enter
Show

Type "# Navigate to your skill directory and scaffold..."
Sleep 800ms
Enter
Sleep 300ms
Type "skill-eval init"
Sleep 400ms
Enter
Sleep 400ms

Type "# It scaffolded evals/evals.json — open it up and fill it in!"
Sleep 1s
Enter
Sleep 300ms
Type "cat evals/evals.json | jq ."
Enter
Sleep 2s

Hide
Type "cat > evals/evals.json << 'ENDJSON'"
Enter
Type `{`
Enter
Type `  "skill_name": "csv-analyzer",`
Enter
Type `  "evals": [`
Enter
Type `    {`
Enter
Type `      "id": 1,`
Enter
Type `      "prompt": "Analyze sales_2025.csv and show me the top 3 months by revenue.",`
Enter
Type `      "expected_output": "A ranked list of the top 3 months with revenue totals.",`
Enter
Type `      "assertions": [`
Enter
Type `        "Output names exactly 3 months",`
Enter
Type `        "Months are sorted by revenue descending",`
Enter
Type `        "Revenue totals are shown"`
Enter
Type `      ]`
Enter
Type `    }`
Enter
Type `  ]`
Enter
Type `}`
Enter
Type "ENDJSON"
Enter
Show

Type "cat evals/evals.json | jq ."
Enter
Sleep 2s

Type "# Looks great! Time to run the full loop..."
Sleep 800ms
Enter
Sleep 300ms
Type "skill-eval loop"
Sleep 400ms
Enter
Sleep 7s
