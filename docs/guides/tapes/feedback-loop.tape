Output docs/site/public/guides/feedback-loop.mp4

Set Shell "bash"
Set FontSize 18
Set Width 1400
Set Height 800
Set Theme "Catppuccin Mocha"
Set Padding 24
Set TypingSpeed 45ms

Hide
Type `ORIG=$(pwd) && skill-eval() { bash "$ORIG/docs/guides/tapes/harness-feedback.sh" "$@"; }`
Enter
Type "WKSP=$(mktemp -d) && cd $WKSP && mkdir -p workspace/iteration-1"
Enter
Type "cat > workspace/iteration-1/grading.json << 'ENDJSON'"
Enter
Type `{`
Enter
Type `  "eval-1": {`
Enter
Type `    "overall": "FAIL",`
Enter
Type `    "assertions": [`
Enter
Type `      { "assertion": "Output includes a bar chart image", "result": "PASS" },`
Enter
Type `      { "assertion": "Chart shows exactly 3 months", "result": "FAIL", "reasoning": "Got 5 months — top-3 filter was not applied" },`
Enter
Type `      { "assertion": "Both axes are labeled", "result": "PASS" }`
Enter
Type `    ]`
Enter
Type `  },`
Enter
Type `  "eval-2": {`
Enter
Type `    "overall": "PASS",`
Enter
Type `    "assertions": [`
Enter
Type `      { "assertion": "CSV written to disk", "result": "PASS" }`
Enter
Type `    ]`
Enter
Type `  }`
Enter
Type `}`
Enter
Type "ENDJSON"
Enter
Type "cat > workspace/iteration-1/feedback.json << 'ENDFB'"
Enter
Type `{}`
Enter
Type "ENDFB"
Enter
Type "clear"
Enter
Show

Type "# After grading, check what the judge found..."
Sleep 800ms
Enter
Sleep 300ms
Type "cat workspace/iteration-1/grading.json | jq ."
Enter
Sleep 2.5s

Type "# eval-1 failed — chart shows all months, not just the top 3."
Sleep 1s
Enter
Sleep 300ms
Type "# Let's leave specific feedback so the next run gets it right!"
Sleep 800ms
Enter
Sleep 400ms

Hide
Type `cat > workspace/iteration-1/feedback.json << 'ENDFB'`
Enter
Type `{`
Enter
Type `  "eval-1": "Chart shows all months, not just top 3 by revenue. The skill should filter before plotting.",`
Enter
Type `  "eval-2": ""`
Enter
Type `}`
Enter
Type "ENDFB"
Enter
Show

Type "cat workspace/iteration-1/feedback.json | jq ."
Enter
Sleep 1.5s

Type "# Feedback locked in! Re-run the grade..."
Sleep 900ms
Enter
Sleep 300ms
Type "skill-eval grade"
Sleep 300ms
Enter
Sleep 1.2s

Type "# The judge sees your notes alongside the output on the next run. 🎯"
Sleep 2s
Enter
