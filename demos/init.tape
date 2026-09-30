# VHS tape: zhi init
#
# Initializes a new zhi workspace with a config provider and store provider.
#
# Prerequisites:
#   - vhs (https://github.com/charmbracelet/vhs)
#   - zhi binary in PATH
#
# Run with: vhs init.tape

Output init.gif

Require zhi

Set Shell zsh
Set FontSize 16
Set Width 1200
Set Height 600
Set Padding 20
Set TypingSpeed 60ms
Set Theme "Catppuccin Mocha"
Set WindowBar Colorful
Set BorderRadius 8
Set Framerate 30
Set CursorBlink true

Type "cd $(mktemp -d) && pwd"
Sleep 500ms
Enter
Sleep 2s

Type "zhi init --config-provider structuredfile --store-provider zhi-store-json"
Sleep 500ms
Enter
Sleep 5s
