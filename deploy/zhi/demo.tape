# VHS tape file for demonstrating zhi as an MCP service with Claude Code
#
# Prerequisites:
#   - vhs (https://github.com/charmbracelet/vhs)
#   - zhi binary in PATH
#   - claude (Claude Code CLI) in PATH
#   - A running Vault server (see deploy/vault/)
#   - Docker with Compose plugin
#
# Run with: vhs demo.tape

Output demo.gif
Output demo.mp4

Require echo
Require claude
Require zhi

Set Shell zsh
Set FontSize 16
Set Width 1800
Set Height 900
Set Padding 20
Set TypingSpeed 60ms
Set Theme "Catppuccin Mocha"
Set WindowBar Colorful
Set BorderRadius 8
Set Framerate 30
Set CursorBlink true

# -- Step 1: Explain what to do ------------------------------------------------

Type `echo 'Run "ZHI_MCP_TOKEN=zhk_example zhi edit" in a separate process!'`
Sleep 500ms
Enter
Sleep 3s

# -- Step 2: Register zhi MCP server with Claude Code --------------------------

Type `claude mcp add --transport http zhi-edit http://127.0.0.1:8091/mcp --header "Authorization: Bearer zhk_example"`
Sleep 500ms
Enter
Sleep 3s

# -- Step 3: Launch Claude Code ------------------------------------------------

Type `echo 'We only skip permissions for this demo recording!'`
Sleep 500ms
Enter
Sleep 1s
Type "claude --dangerously-skip-permissions"
Sleep 500ms
Enter
Sleep 3s
Down
Enter
Sleep 4s

# -- Step 4: Show fast mode is off ----------------------------------------------

Type "/fast"
Sleep 500ms
Enter
Sleep 3s
Escape

# -- Step 5: Verify MCP connection ---------------------------------------------

Type "/mcp"
Sleep 500ms
Enter
Sleep 2s
Enter
Sleep 3s
Escape
Sleep 500ms
Escape
Sleep 2s

# -- Step 6: Login to Vault via MCP store_login --------------------------------

Type `Use the store_login tool to login with method "approle" and credentials "role_id=zhi_mcp" and "secret_id=a7f3976e-5ec4-50a6-05e2-98413a6a394e".`
Sleep 500ms
Enter
Sleep 20s

# -- Step 7: Review current configuration --------------------------------------

Type "/zhi-edit:review-config (MCP)"
Sleep 500ms
Enter
Sleep 22s

# -- Step 8: Make configuration changes ----------------------------------------

Type `Set the zhi-marketplace/port to use 8090 and add api-keys for provider "my-example" with key "zhk_example".`
Sleep 500ms
Enter
Sleep 20s

# -- Step 9: Apply changes (export + docker compose up) ------------------------

Type "/zhi-edit:apply-changes (MCP)"
Sleep 500ms
Enter
Sleep 40s
Type "The error in the store is just to show that error handling is also possible with this MCP server. Zhiya!"
Sleep 2s
