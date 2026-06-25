Output docs/site/public/guides/auto-fixing.mp4

Set Shell "bash"
Set FontSize 18
Set Width 1400
Set Height 800
Set Theme "Catppuccin Mocha"
Set Padding 24
Set TypingSpeed 35ms

Hide
Type `ORIG=$(pwd) && skill-eval() { bash "$ORIG/docs/guides/tapes/harness-auto-fix.sh" "$@"; }`
Enter
Type "clear"
Enter
Show

Type "# Let's run the full loop with auto-fixing enabled..."
Sleep 800ms
Enter
Sleep 300ms
Type "skill-eval loop --fix --max-fix-attempts 2"
Sleep 400ms
Enter
Sleep 9s

Type "# eval 2 went from 1/2 passed → 2/2 passed in two fix attempts! 🎉"
Sleep 1s
Enter
Sleep 400ms
Type "# The judge's reasoning fed right back to the agent as a critique."
Sleep 1.2s
Enter
Sleep 300ms
Type "# That's auto-refinement in action. 🪄"
Sleep 1.5s
Enter
