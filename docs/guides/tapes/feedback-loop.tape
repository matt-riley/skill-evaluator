Output docs/site/public/guides/feedback-loop.gif

Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"
Set Padding 24
Set TypingSpeed 40ms

# --- silent workspace setup using python to avoid quoting issues ---
Hide
Type "WKSP=$(mktemp -d) && cd $WKSP && mkdir -p workspace/iteration-1"
Enter
Type "python3 -c \""
Enter
Type "import json, pathlib"
Enter
Type "g = {'eval-1': {'overall': 'FAIL', 'assertions': [{'assertion': 'Output includes a bar chart image', 'result': 'PASS'}, {'assertion': 'Chart shows exactly 3 months', 'result': 'FAIL', 'reasoning': 'Got 5 months — top-3 filter was not applied'}, {'assertion': 'Both axes are labeled', 'result': 'PASS'}]}, 'eval-2': {'overall': 'PASS', 'assertions': [{'assertion': 'CSV written to disk', 'result': 'PASS'}]}}"
Enter
Type "pathlib.Path('workspace/iteration-1/grading.json').write_text(json.dumps(g, indent=2))"
Enter
Type "pathlib.Path('workspace/iteration-1/feedback.json').write_text(json.dumps({'eval-1': '', 'eval-2': ''}, indent=2))"
Enter
Type "\""
Enter
Show

Type "# Let's check what the grader found after running..."
Sleep 700ms
Enter
Sleep 400ms
Type "cat workspace/iteration-1/grading.json | jq ."
Enter
Sleep 2s

Type "# eval-1 failed — chart is showing all months, not just the top 3."
Sleep 900ms
Enter
Sleep 400ms
Type "# Time to leave specific feedback so the next run gets it right!"
Sleep 700ms
Enter
Sleep 400ms

Hide
Type "python3 -c \""
Enter
Type "import json, pathlib"
Enter
Type "fb = {'eval-1': 'Chart shows all months, not just top 3 by revenue. Skill should filter before plotting.', 'eval-2': ''}"
Enter
Type "pathlib.Path('workspace/iteration-1/feedback.json').write_text(json.dumps(fb, indent=2))"
Enter
Type "\""
Enter
Show

Type "cat workspace/iteration-1/feedback.json | jq ."
Enter
Sleep 1.5s

Type "# Feedback locked in! Next: skill-eval grade re-runs the judge"
Sleep 900ms
Enter
Sleep 300ms
Type "# — and the judge will see your notes alongside the output. 🎯"
Sleep 1.5s
Enter
