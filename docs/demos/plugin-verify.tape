# VHS tape: zhi plugin verify
#
# Verifies a plugin artifact's cosign signature without installing it.
# Useful for auditing and compliance checks.
#
# Prerequisites:
#   - vhs (https://github.com/charmbracelet/vhs)
#   - zhi binary in PATH
#   - Network access to ghcr.io
#
# Run with: vhs plugin-verify.tape

Output plugin-verify.gif

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

Type "zhi plugin verify oci://ghcr.io/mrwong99/zhi/zhi-config-pokedex:v0.0.9"
Sleep 500ms
Enter
Sleep 8s
