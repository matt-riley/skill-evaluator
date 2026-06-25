Output docs/site/public/guides/cross-model.mp4

Set Shell "bash"
Set FontSize 18
Set Width 1400
Set Height 800
Set Theme "Catppuccin Mocha"
Set Padding 24
Set TypingSpeed 35ms

Hide
Type `ORIG=$(pwd) && skill-eval() { bash "$ORIG/docs/guides/tapes/harness-cross-model.sh" "$@"; }`
Enter
Type "clear"
Enter
Show

Type "# Let's see how our skill performs across three different agents..."
Sleep 1s
Enter
Sleep 300ms
Type "skill-eval loop --models pi:claude-sonnet,claude,copilot"
Sleep 400ms
Enter
Sleep 14s

Type "# pi got +33%, copilot +33%, claude only +17%. Time to strengthen the skill for Claude! 🔍"
Sleep 2s
Enter
