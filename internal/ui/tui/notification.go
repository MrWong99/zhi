package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	internalui "github.com/MrWong99/zhi/internal/ui"
	zhiui "github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// updatesAvailableMsg is sent when update check completes.
type updatesAvailableMsg struct {
	updates []zhiui.PluginUpdate
	err     error
}

// Notification displays a non-intrusive update notification.
type Notification struct {
	updates   []zhiui.PluginUpdate
	visible   bool
	dismissed bool
	width     int
}

// NewNotification creates a notification component.
func NewNotification() Notification {
	return Notification{width: 80}
}

// SetWidth updates the notification width.
func (n *Notification) SetWidth(width int) {
	n.width = width
}

// CheckForUpdates returns a tea.Cmd that checks for plugin updates.
func CheckForUpdates(ctx context.Context, ctrl *internalui.UIController) tea.Cmd {
	return func() tea.Msg {
		updates, err := ctrl.CheckUpdates(ctx)
		return updatesAvailableMsg{updates: updates, err: err}
	}
}

// UpdateNotification handles messages for the notification component.
func (n Notification) UpdateNotification(msg tea.Msg) (Notification, tea.Cmd) {
	switch msg := msg.(type) {
	case updatesAvailableMsg:
		if msg.err == nil && len(msg.updates) > 0 {
			n.updates = msg.updates
			n.visible = true
		}
		return n, nil

	case tea.KeyMsg:
		if n.visible {
			switch msg.String() {
			case "d":
				n.visible = false
				n.dismissed = true
				return n, nil
			}
		}
	}
	return n, nil
}

// Visible returns whether the notification is currently showing.
func (n Notification) Visible() bool {
	return n.visible && !n.dismissed
}

// View renders the notification overlay.
func (n Notification) View() string {
	if !n.Visible() || len(n.updates) == 0 {
		return ""
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "  %d plugin update(s) available:\n", len(n.updates))
	for _, u := range n.updates {
		fmt.Fprintf(&sb, "    %s: %s -> %s\n", u.Name, u.CurrentVersion, u.LatestVersion)
	}
	sb.WriteString("  [d]ismiss")
	sb.WriteString("\n")

	return InfoStyle.Render(sb.String())
}
