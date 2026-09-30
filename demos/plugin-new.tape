# VHS tape: zhi plugin new
#
# Scaffolds a complete new plugin project from a template.
#
# Prerequisites:
#   - vhs (https://github.com/charmbracelet/vhs)
#   - zhi binary in PATH
#
# Run with: vhs plugin-new.tape

Output plugin-new.gif

Require zhi

Set Shell zsh
Set FontSize 16
Set Width 1400
Set Height 700
Set Padding 20
Set TypingSpeed 60ms
Set Theme "Catppuccin Mocha"
Set WindowBar Colorful
Set BorderRadius 8
Set Framerate 30
Set CursorBlink true

Type "cd $(mktemp -d)"
Sleep 500ms
Enter
Sleep 1s

Type "zhi plugin new --name my-store --type store --author demo --license MIT"
Sleep 500ms
Enter
Sleep 6s
