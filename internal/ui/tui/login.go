package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/MrWong99/zhi/internal/core/authcallback"
	"github.com/MrWong99/zhi/internal/ui"
	zhiui "github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// loginPhase tracks which part of the login flow is active.
type loginPhase int

const (
	loginPhaseMethod loginPhase = iota
	loginPhaseForm
	loginPhaseInteractive // waiting for browser-based auth callback
)

// loginResultMsg is sent after a login attempt completes.
type loginResultMsg struct {
	session *zhiui.StoreSession
	err     error
}

// loginModel is the Bubbletea model for the store login view.
type loginModel struct {
	controller *ui.UIController
	ctx        context.Context
	methods    []zhiui.StoreAuthMethod
	phase      loginPhase
	// Method selection state.
	methodCursor int
	// Credential form state.
	selectedMethod zhiui.StoreAuthMethod
	inputs         []textinput.Model
	focusIndex     int
	// Result state.
	submitting bool
	errMsg     string
	message    string // e.g. "Session expired. Please log in again."
	width      int
	height     int
}

// newLoginModel creates a login model with the given auth methods.
func newLoginModel(ctx context.Context, controller *ui.UIController, methods []zhiui.StoreAuthMethod, message string) loginModel {
	m := loginModel{
		controller: controller,
		ctx:        ctx,
		methods:    methods,
		message:    message,
		width:      80,
		height:     24,
	}

	if len(methods) == 1 {
		m.selectedMethod = methods[0]
		if m.selectedMethod.Interactive {
			// Interactive single method: go straight to interactive phase.
			// The interactiveLoginCmd is triggered via Init(); mark it as
			// in flight so its loginResultMsg (including errors) is handled.
			m.phase = loginPhaseInteractive
			m.submitting = true
		} else {
			m.phase = loginPhaseForm
			m.buildInputs()
		}
	} else {
		m.phase = loginPhaseMethod
	}

	return m
}

func (m *loginModel) buildInputs() {
	m.inputs = make([]textinput.Model, len(m.selectedMethod.Fields))
	for i, field := range m.selectedMethod.Fields {
		ti := textinput.New()
		ti.Placeholder = field.Description
		ti.CharLimit = 256
		ti.Width = 40
		if field.Secret {
			ti.EchoMode = textinput.EchoNone
			ti.EchoCharacter = '•'
		}
		if i == 0 {
			ti.Focus()
		}
		m.inputs[i] = ti
	}
	m.focusIndex = 0
}

// SetSize updates the dimensions available for rendering.
func (m *loginModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Init returns the initial command for the login model.
func (m loginModel) Init() tea.Cmd {
	if m.phase == loginPhaseInteractive {
		return m.interactiveLoginCmd()
	}
	if m.phase == loginPhaseForm && len(m.inputs) > 0 {
		return textinput.Blink
	}
	return nil
}

// UpdateLogin handles messages for the login view.
func (m loginModel) UpdateLogin(msg tea.Msg) (loginModel, tea.Cmd) {
	// Always consume the login result, regardless of phase or submitting
	// state, so an error can never be dropped on the floor (which would
	// leave the view stuck on "Waiting for browser authentication...").
	if result, ok := msg.(loginResultMsg); ok {
		m.submitting = false
		if result.err != nil {
			m.errMsg = result.err.Error()
		}
		// Success is handled by the app model checking the result message.
		return m, nil
	}
	if m.submitting {
		// Ignore all other input while a login attempt is in flight.
		return m, nil
	}

	switch m.phase {
	case loginPhaseMethod:
		return m.updateMethodSelection(msg)
	case loginPhaseForm:
		return m.updateCredentialForm(msg)
	case loginPhaseInteractive:
		return m.updateInteractive(msg)
	}
	return m, nil
}

func (m loginModel) updateMethodSelection(msg tea.Msg) (loginModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "j", "down":
			if m.methodCursor < len(m.methods)-1 {
				m.methodCursor++
			}
		case "k", "up":
			if m.methodCursor > 0 {
				m.methodCursor--
			}
		case "enter":
			m.selectedMethod = m.methods[m.methodCursor]
			m.errMsg = ""
			if m.selectedMethod.Interactive {
				return m.startInteractiveLogin()
			}
			m.phase = loginPhaseForm
			m.buildInputs()
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m loginModel) updateCredentialForm(msg tea.Msg) (loginModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "tab", "shift+tab":
			if len(m.inputs) == 0 {
				// A method with no fields (e.g. ambient credentials) has an
				// empty form; skip focus cycling to avoid a divide by zero.
				return m, nil
			}
			dir := 1
			if keyMsg.String() == "shift+tab" {
				dir = -1
			}
			m.focusIndex = (m.focusIndex + dir + len(m.inputs)) % len(m.inputs)
			cmds := make([]tea.Cmd, len(m.inputs))
			for i := range m.inputs {
				if i == m.focusIndex {
					cmds[i] = m.inputs[i].Focus()
				} else {
					m.inputs[i].Blur()
				}
			}
			return m, tea.Batch(cmds...)

		case "enter":
			return m.submitLogin()

		case "esc":
			if len(m.methods) > 1 {
				m.phase = loginPhaseMethod
				m.errMsg = ""
				return m, nil
			}
		}
	}

	// Update the focused input.
	if m.focusIndex < len(m.inputs) {
		var cmd tea.Cmd
		m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m loginModel) submitLogin() (loginModel, tea.Cmd) {
	credentials := make(map[string]string, len(m.inputs))
	for i, field := range m.selectedMethod.Fields {
		credentials[field.Name] = m.inputs[i].Value()
	}
	method := m.selectedMethod.Type
	m.submitting = true
	m.errMsg = ""

	ctx := m.ctx
	ctrl := m.controller
	return m, func() tea.Msg {
		session, err := ctrl.StoreLogin(ctx, method, credentials)
		return loginResultMsg{session: session, err: err}
	}
}

func (m loginModel) startInteractiveLogin() (loginModel, tea.Cmd) {
	m.phase = loginPhaseInteractive
	m.submitting = true
	m.errMsg = ""
	return m, m.interactiveLoginCmd()
}

func (m loginModel) interactiveLoginCmd() tea.Cmd {
	ctx := m.ctx
	ctrl := m.controller
	method := m.selectedMethod.Type

	return func() tea.Msg {
		// Start callback server.
		cbSrv, err := authcallback.NewCallbackServer()
		if err != nil {
			return loginResultMsg{err: fmt.Errorf("failed to start callback server: %w", err)}
		}
		defer cbSrv.Close()

		// Initiate interactive login with callback URL.
		chall, err := ctrl.StoreLoginInteractive(ctx, method, map[string]string{
			"redirect_uri": cbSrv.URL(),
		})
		if err != nil {
			return loginResultMsg{err: fmt.Errorf("failed to start interactive login: %w", err)}
		}

		// Open browser.
		openBrowser(chall.AuthURL)

		// Wait for callback.
		result, err := cbSrv.Wait(ctx)
		if err != nil {
			return loginResultMsg{err: fmt.Errorf("authentication callback failed: %w", err)}
		}

		// Complete the login.
		session, err := ctrl.StoreLoginInteractiveCallback(ctx, chall.ChallengeID, result.Params)
		return loginResultMsg{session: session, err: err}
	}
}

func (m loginModel) updateInteractive(msg tea.Msg) (loginModel, tea.Cmd) {
	// While waiting for browser callback, allow esc to go back (if multiple
	// methods) and 'r' to retry a failed attempt.
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			if len(m.methods) > 1 {
				m.phase = loginPhaseMethod
				m.submitting = false
				m.errMsg = ""
				return m, nil
			}
		case "r":
			if !m.submitting {
				m.submitting = true
				m.errMsg = ""
				return m, m.interactiveLoginCmd()
			}
		}
	}
	return m, nil
}

// openBrowser opens the given URL in the user's default browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// View renders the login view.
func (m loginModel) View() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(HeaderStyle.Render(" Store Login "))
	sb.WriteString("\n\n")

	if m.message != "" {
		sb.WriteString(WarningStyle.Render(fmt.Sprintf("  %s", m.message)))
		sb.WriteString("\n\n")
	} else {
		sb.WriteString(DimStyle.Render("  The store requires authentication."))
		sb.WriteString("\n\n")
	}

	switch m.phase {
	case loginPhaseMethod:
		sb.WriteString(m.viewMethodSelection())
	case loginPhaseForm:
		sb.WriteString(m.viewCredentialForm())
	case loginPhaseInteractive:
		sb.WriteString(m.viewInteractive())
	}

	if m.errMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(ErrorStyle.Render(fmt.Sprintf("  Error: %s", m.errMsg)))
		sb.WriteString("\n")
	}

	if m.submitting {
		sb.WriteString("\n")
		sb.WriteString(SpinnerStyle.Render("  Logging in..."))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(m.viewHints())

	return sb.String()
}

func (m loginModel) viewMethodSelection() string {
	var sb strings.Builder
	sb.WriteString("  Select authentication method:\n\n")
	for i, method := range m.methods {
		cursor := "  "
		if i == m.methodCursor {
			cursor = "> "
		}
		style := ValueStyle
		if i == m.methodCursor {
			style = ActiveStyle
		}
		line := method.Type
		if method.Interactive {
			line += " (opens browser)"
		}
		if method.Description != "" {
			line += DimStyle.Render(fmt.Sprintf(" - %s", method.Description))
		}
		fmt.Fprintf(&sb, "  %s%s\n", cursor, style.Render(line))
	}
	return sb.String()
}

func (m loginModel) viewInteractive() string {
	var sb strings.Builder
	if len(m.methods) > 1 {
		sb.WriteString(DimStyle.Render(fmt.Sprintf("  Method: %s", m.selectedMethod.Type)))
		sb.WriteString("\n\n")
	}
	if m.submitting {
		sb.WriteString("  Waiting for browser authentication...\n")
		sb.WriteString(DimStyle.Render("  A browser window should open. Complete the login there."))
		sb.WriteString("\n")
	} else {
		sb.WriteString("  Interactive login did not complete.\n")
		sb.WriteString(DimStyle.Render("  Press r to try again."))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (m loginModel) viewCredentialForm() string {
	var sb strings.Builder

	if len(m.methods) > 1 {
		sb.WriteString(DimStyle.Render(fmt.Sprintf("  Method: %s", m.selectedMethod.Type)))
		sb.WriteString("\n\n")
	}

	for i, field := range m.selectedMethod.Fields {
		label := field.Name
		if field.Required {
			label += RequiredBadgeStyle.Render(" *")
		}
		fmt.Fprintf(&sb, "  %s:\n", label)
		fmt.Fprintf(&sb, "  %s\n\n", m.inputs[i].View())
	}
	return sb.String()
}

func (m loginModel) viewHints() string {
	switch m.phase {
	case loginPhaseMethod:
		return DimStyle.Render("  j/k: select  Enter: choose  Ctrl+C: quit")
	case loginPhaseForm:
		hints := "  Tab: next field  Enter: login  Ctrl+C: quit"
		if len(m.methods) > 1 {
			hints = "  Tab: next field  Enter: login  Esc: back  Ctrl+C: quit"
		}
		return DimStyle.Render(hints)
	case loginPhaseInteractive:
		var parts []string
		if !m.submitting {
			parts = append(parts, "r: retry")
		}
		if len(m.methods) > 1 {
			parts = append(parts, "Esc: back")
		}
		parts = append(parts, "Ctrl+C: quit")
		return DimStyle.Render("  " + strings.Join(parts, "  "))
	}
	return ""
}
