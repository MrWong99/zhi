# VHS tape: zhi validate
#
# Validates the configuration tree and prints results grouped by severity.
#
# Prerequisites:
#   - vhs (https://github.com/charmbracelet/vhs)
#   - zhi binary in PATH
#   - A zhi workspace (run `zhi init` first)
#
# Run with: vhs validate.tape

Output validate.gif

Require zhi

Set Shell zsh
Set FontSize 16
Set Width 1200
Set Height 500
Set Padding 20
Set TypingSpeed 60ms
Set Theme "Catppuccin Mocha"
Set WindowBar Colorful
Set BorderRadius 8
Set Framerate 30
Set CursorBlink true

# Set up a temporary workspace
Type "cd $(mktemp -d) && zhi init > /dev/null"
Sleep 500ms
Enter
Sleep 3s

Type "zhi validate"
Sleep 500ms
Enter
Sleep 5s
