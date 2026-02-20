# VHS tape: zhi plugin install
#
# Installs a plugin from an OCI registry (ghcr.io).
#
# Prerequisites:
#   - vhs (https://github.com/charmbracelet/vhs)
#   - zhi binary in PATH
#   - Network access to ghcr.io
#
# Run with: vhs plugin-install.tape

Output plugin-install.gif

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

Type "zhi plugin install oci://ghcr.io/mrwong99/zhi/zhi-store-memory:v0.0.9"
Sleep 500ms
Enter
Sleep 8s
