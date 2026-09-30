# VHS tape: zhi plugin list
#
# Lists all installed shared plugins with version and source info.
#
# Prerequisites:
#   - vhs (https://github.com/charmbracelet/vhs)
#   - zhi binary in PATH
#   - At least one plugin installed (e.g. via `zhi plugin install`)
#
# Run with: vhs plugin-list.tape

Output plugin-list.gif

Require zhi

Set Shell zsh
Set FontSize 16
Set Width 1400
Set Height 500
Set Padding 20
Set TypingSpeed 60ms
Set Theme "Catppuccin Mocha"
Set WindowBar Colorful
Set BorderRadius 8
Set Framerate 30
Set CursorBlink true

Type "zhi plugin list"
Sleep 500ms
Enter
Sleep 5s
