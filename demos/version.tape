# VHS tape: zhi version
#
# Shows the zhi CLI version, commit hash, and build date.
#
# Prerequisites:
#   - vhs (https://github.com/charmbracelet/vhs)
#   - zhi binary in PATH
#
# Run with: vhs version.tape

Output version.gif

Require zhi

Set Shell zsh
Set FontSize 16
Set Width 900
Set Height 300
Set Padding 20
Set TypingSpeed 60ms
Set Theme "Catppuccin Mocha"
Set WindowBar Colorful
Set BorderRadius 8
Set Framerate 30
Set CursorBlink true

Type "zhi version"
Sleep 500ms
Enter
Sleep 4s
