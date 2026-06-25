Output docs/site/public/guides/first-eval.mp4

Set Shell "bash"
Set FontSize 18
Set Width 1400
Set Height 800
Set Theme "Catppuccin Mocha"
Set Padding 24
Set TypingSpeed 40ms

Hide
Type "WKSP=$(mktemp -d)/my-skill && mkdir -p $WKSP && cd $WKSP"
Enter
Type "mkdir -p evals"
Enter
Type "cat > .skill-eval.yaml << 'EOF'"
Enter
Type "agent: pi"
Enter
Type "model: claude-sonnet-4-5"
Enter
Type "EOF"
Enter
Show

Type "# We're inside our skill directory. Let's scaffold!"
Sleep 700ms
Enter
Sleep 300ms
Type "# skill-eval init  →  creates evals/evals.json"
Sleep 800ms
Enter
Sleep 400ms

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
Sleep 400ms
Type "# skill-eval loop  →  run → grade → benchmark"
Sleep 1.5s
Enter
