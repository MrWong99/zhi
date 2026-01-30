package tui

import "github.com/MrWong99/zhi/internal/ui"

func init() {
	ui.Register("tui", func() ui.UIDriver {
		return &TUIDriver{}
	})
}
