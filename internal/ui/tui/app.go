package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/MrWong99/zhi/internal/ui"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	zhiui "github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// viewType identifies which sub-view is currently active.
type viewType int

const (
	viewLogin viewType = iota
	viewTree
	viewEditor
	viewComponent
	viewValidation
	viewApply
	viewExport
	viewMarketplace
	viewInstalled
	viewPluginDetail
)

// TUIDriver implements ui.UIDriver using Bubbletea.
type TUIDriver struct{}

// Run starts the Bubbletea TUI and blocks until exit.
func (d *TUIDriver) Run(ctx context.Context, controller *ui.UIController) error {
	app, err := NewApp(ctx, controller)
	if err != nil {
		return fmt.Errorf("initializing TUI: %w", err)
	}

	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithContext(ctx))
	_, err = p.Run()
	return err
}

// treeLoadedMsg is sent when the tree has been loaded.
type treeLoadedMsg struct {
	tree *config.Tree
	err  error
}

// statusMsg is sent to display a status bar message.
type statusMsg string

// errMsg is sent when an error occurs.
type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

// App is the root Bubbletea model managing the TUI state and view routing.
type App struct {
	controller       *ui.UIController
	ctx              context.Context
	tree             *config.Tree
	activeView       viewType
	previousView     viewType // view to return to after re-authentication
	loginView        loginModel
	treeView         TreeView
	editorView       ValueEditor
	componentView    ComponentView
	validationView   ValidationView
	applyView        ApplyView
	exportView       ExportView
	marketplaceView  MarketplaceView
	installedView    InstalledView
	pluginDetailView PluginDetailView
	notification     Notification
	statusMsg        string
	err              error
	width            int
	height           int
	showHelp         bool
}

// NewApp creates a new App and performs the initial tree load.
func NewApp(ctx context.Context, controller *ui.UIController) (*App, error) {
	app := &App{
		controller: controller,
		ctx:        ctx,
		activeView: viewTree,
		width:      80,
		height:     24,
	}

	// Check if authentication is required before loading the tree.
	if needsLogin, methods := app.checkAuthRequired(ctx); needsLogin {
		app.loginView = newLoginModel(ctx, controller, methods, "")
		app.activeView = viewLogin
		app.notification = NewNotification()
		return app, nil
	}

	tree, err := controller.LoadTree(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading initial tree: %w", err)
	}

	app.tree = tree
	app.treeView = NewTreeView(controller, tree)
	app.editorView = NewValueEditor()
	app.componentView = NewComponentView(controller)
	app.validationView = NewValidationView()
	app.applyView = NewApplyView()
	app.exportView = NewExportView(controller)
	app.marketplaceView = NewMarketplaceView(ctx, controller)
	app.installedView = NewInstalledView(ctx, controller)
	app.notification = NewNotification()

	return app, nil
}

// checkAuthRequired checks if the store requires authentication.
// Returns true and the available methods if login is needed.
func (a *App) checkAuthRequired(ctx context.Context) (bool, []zhiui.StoreAuthMethod) {
	methods, err := a.controller.StoreAuthMethods(ctx)
	if err != nil || len(methods) == 0 {
		return false, nil
	}
	session, err := a.controller.StoreAuthStatus(ctx)
	if err != nil || session == nil {
		return false, nil
	}
	if session.Status != zhiui.StoreSessionUnauthenticated && session.Status != zhiui.StoreSessionExpired {
		return false, nil
	}
	return true, methods
}

// showLoginView transitions to the login view with an optional message.
func (a *App) showLoginView(message string) tea.Cmd {
	methods, err := a.controller.StoreAuthMethods(a.ctx)
	if err != nil || len(methods) == 0 {
		a.statusMsg = "Authentication required but no auth methods available"
		return nil
	}
	a.previousView = a.activeView
	a.loginView = newLoginModel(a.ctx, a.controller, methods, message)
	a.loginView.SetSize(a.width, a.contentHeight())
	a.activeView = viewLogin
	return a.loginView.Init()
}

// Init returns the initial command.
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		tea.WindowSize(),
		a.applyView.Init(),
	)
}

// Update handles messages and routes them to the active view.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.loginView.SetSize(a.width, a.contentHeight())
		a.treeView.SetSize(a.width, a.contentHeight())
		a.validationView.SetSize(a.width, a.contentHeight())
		a.applyView.SetSize(a.width, a.contentHeight())
		a.exportView.SetSize(a.width, a.contentHeight())
		a.componentView.SetSize(a.width, a.contentHeight())
		a.marketplaceView.SetSize(a.width, a.contentHeight())
		a.installedView.SetSize(a.width, a.contentHeight())
		a.pluginDetailView.SetSize(a.width, a.contentHeight())
		a.notification.SetWidth(a.width)
		return a, nil

	case tea.KeyMsg:
		// Global key bindings.
		switch msg.String() {
		case "ctrl+c":
			if a.activeView == viewApply && a.applyView.running {
				// Cancel apply command.
				var cmd tea.Cmd
				a.applyView, cmd = a.applyView.UpdateApply(msg)
				return a, cmd
			}
			return a, tea.Quit
		case "q":
			if a.activeView == viewTree && !a.treeView.filtering {
				return a, tea.Quit
			}
			if a.activeView == viewApply && !a.applyView.running {
				a.activeView = viewTree
				a.statusMsg = ""
				return a, nil
			}
		case "?":
			if a.activeView == viewTree || a.activeView == viewComponent {
				a.showHelp = !a.showHelp
				return a, nil
			}
		case "esc":
			if a.showHelp {
				a.showHelp = false
				return a, nil
			}
			if a.activeView == viewLogin {
				// Let the login view handle Esc (e.g. back to method selection).
				break
			}
			if a.activeView == viewTree {
				if a.treeView.filtering {
					a.treeView.ClearFilter()
					return a, nil
				}
			}
			if a.activeView != viewTree {
				if a.activeView == viewApply && a.applyView.running {
					return a, nil // Can't leave apply while running
				}
				a.activeView = viewTree
				a.statusMsg = ""
				a.refreshTreeView()
				return a, nil
			}
		}

	case loginResultMsg:
		if msg.err != nil {
			// Error is displayed within the login view itself.
			a.loginView, _ = a.loginView.UpdateLogin(msg)
			return a, nil
		}
		a.loginView, _ = a.loginView.UpdateLogin(msg)
		// Successful login: load tree and transition to tree view.
		a.statusMsg = "Login successful"
		return a, func() tea.Msg {
			tree, err := a.controller.LoadTree(a.ctx)
			return treeLoadedMsg{tree: tree, err: err}
		}

	case treeLoadedMsg:
		if msg.err != nil {
			if store.IsAuthRequired(msg.err) {
				return a, a.showLoginView("Authentication required. Please log in.")
			}
			a.err = msg.err
			a.statusMsg = "Error loading tree: " + msg.err.Error()
			return a, nil
		}
		a.tree = msg.tree
		a.treeView = NewTreeView(a.controller, a.tree)
		a.treeView.SetSize(a.width, a.contentHeight())
		if a.activeView == viewLogin {
			// Transitioning from login: initialize all views and go to tree.
			a.editorView = NewValueEditor()
			a.componentView = NewComponentView(a.controller)
			a.validationView = NewValidationView()
			a.applyView = NewApplyView()
			a.exportView = NewExportView(a.controller)
			a.marketplaceView = NewMarketplaceView(a.ctx, a.controller)
			a.installedView = NewInstalledView(a.ctx, a.controller)
			a.activeView = viewTree
			a.statusMsg = "Login successful"
		} else {
			a.statusMsg = "Tree reloaded"
		}
		return a, nil

	case statusMsg:
		a.statusMsg = string(msg)
		return a, nil

	case errMsg:
		a.err = msg.err
		a.statusMsg = "Error: " + msg.err.Error()
		return a, nil
	}

	// Route to active view.
	switch a.activeView {
	case viewLogin:
		return a.updateLoginView(msg)
	case viewTree:
		return a.updateTreeView(msg)
	case viewEditor:
		return a.updateEditorView(msg)
	case viewComponent:
		return a.updateComponentView(msg)
	case viewValidation:
		return a.updateValidationView(msg)
	case viewApply:
		return a.updateApplyView(msg)
	case viewExport:
		return a.updateExportView(msg)
	case viewMarketplace:
		return a.updateMarketplaceView(msg)
	case viewInstalled:
		return a.updateInstalledView(msg)
	case viewPluginDetail:
		return a.updatePluginDetailView(msg)
	}

	return a, nil
}

func (a *App) updateLoginView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	a.loginView, cmd = a.loginView.UpdateLogin(msg)
	return a, cmd
}

func (a *App) updateTreeView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && !a.treeView.filtering {
		switch keyMsg.String() {
		case "enter":
			path := a.treeView.SelectedPath()
			if path != "" {
				val, ok := a.controller.GetValue(path)
				if ok {
					comp, _ := a.controller.PathBelongsToComponent(path)
					isDisabled := comp != "" && !a.controller.IsComponentEnabled(comp)
					a.editorView = NewValueEditorFor(path, &val, comp, isDisabled)
					a.activeView = viewEditor
					return a, a.editorView.Init()
				}
			}
			return a, nil

		case "s":
			err := a.controller.SaveTree(a.ctx)
			if err != nil {
				if store.IsAuthRequired(err) {
					return a, a.showLoginView("Session expired. Please log in again.")
				}
				if store.IsCASConflict(err) {
					a.statusMsg = "Save conflict: tree was modified externally. Press r to reload."
				} else {
					a.statusMsg = "Save failed: " + err.Error()
				}
			} else {
				a.statusMsg = "Tree saved"
			}
			return a, nil

		case "v":
			results, err := a.controller.Validate(a.ctx)
			if err != nil {
				if store.IsAuthRequired(err) {
					return a, a.showLoginView("Session expired. Please log in again.")
				}
				a.statusMsg = "Validation error: " + err.Error()
				return a, nil
			}
			a.validationView = NewValidationViewWith(results)
			a.validationView.SetSize(a.width, a.contentHeight())
			a.activeView = viewValidation
			return a, nil

		case "a":
			a.applyView = NewApplyView()
			a.applyView.SetSize(a.width, a.contentHeight())
			a.activeView = viewApply
			cmd := a.applyView.StartApply(a.ctx, a.controller)
			return a, cmd

		case "e":
			a.exportView = NewExportView(a.controller)
			a.exportView.SetSize(a.width, a.contentHeight())
			a.activeView = viewExport
			return a, nil

		case "c":
			a.componentView = NewComponentView(a.controller)
			a.componentView.SetSize(a.width, a.contentHeight())
			a.activeView = viewComponent
			return a, nil

		case "r":
			a.statusMsg = "Reloading..."
			return a, func() tea.Msg {
				tree, err := a.controller.LoadTree(a.ctx)
				return treeLoadedMsg{tree: tree, err: err}
			}

		case "m":
			a.marketplaceView = NewMarketplaceView(a.ctx, a.controller)
			a.marketplaceView.SetSize(a.width, a.contentHeight())
			a.activeView = viewMarketplace
			return a, a.marketplaceView.Init()

		case "p":
			a.installedView = NewInstalledView(a.ctx, a.controller)
			a.installedView.SetSize(a.width, a.contentHeight())
			a.activeView = viewInstalled
			return a, a.installedView.Init()
		}
	}

	var cmd tea.Cmd
	a.treeView, cmd = a.treeView.UpdateTree(msg)
	return a, cmd
}

func (a *App) updateEditorView(msg tea.Msg) (tea.Model, tea.Cmd) {
	// When confirming, delegate all input to the editor.
	if a.editorView.IsConfirming() {
		var cmd tea.Cmd
		a.editorView, cmd = a.editorView.UpdateEditor(msg)
		// If confirmation was declined (dirty reset to false), go back.
		if !a.editorView.IsConfirming() && !a.editorView.dirty {
			a.statusMsg = "Change cancelled"
			a.activeView = viewTree
			return a, nil
		}
		// If confirmation accepted, commit the value.
		if !a.editorView.IsConfirming() && a.editorView.dirty {
			return a.commitEditorValue()
		}
		return a, cmd
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		// For collection editors (map/list), delegate enter/esc to the editor
		// when the user is editing an entry inline or navigating entries.
		if a.editorView.IsCollectionEditor() {
			switch keyMsg.String() {
			case "esc":
				if a.editorView.IsEditingInline() {
					// Cancel inline edit, stay in editor.
					var cmd tea.Cmd
					a.editorView, cmd = a.editorView.UpdateEditor(msg)
					return a, cmd
				}
				// Not editing inline: leave the editor.
				a.activeView = viewTree
				a.statusMsg = ""
				return a, nil
			case "enter":
				if a.editorView.IsEditingInline() {
					// Confirm inline edit.
					var cmd tea.Cmd
					a.editorView, cmd = a.editorView.UpdateEditor(msg)
					return a, cmd
				}
				// Not editing inline: commit the collection value.
				if a.editorView.dirty {
					if a.editorView.NeedsConfirmation() {
						a.editorView.StartConfirmation()
						return a, nil
					}
					return a.commitEditorValue()
				}
				a.activeView = viewTree
				return a, nil
			}
			// All other keys: delegate to editor.
			var cmd tea.Cmd
			a.editorView, cmd = a.editorView.UpdateEditor(msg)
			return a, cmd
		}

		switch keyMsg.String() {
		case "esc":
			a.activeView = viewTree
			a.statusMsg = ""
			return a, nil
		case "enter":
			if a.editorView.dirty {
				// Block save if pattern validation fails.
				if a.editorView.HasPatternError() {
					a.statusMsg = "Fix pattern errors before saving"
					return a, nil
				}
				// Check if confirmation is required.
				if a.editorView.NeedsConfirmation() {
					a.editorView.StartConfirmation()
					return a, nil
				}
				return a.commitEditorValue()
			}
			a.activeView = viewTree
			return a, nil
		}
	}

	var cmd tea.Cmd
	a.editorView, cmd = a.editorView.UpdateEditor(msg)
	return a, cmd
}

func (a *App) commitEditorValue() (tea.Model, tea.Cmd) {
	val := a.editorView.CommitValue()
	err := a.controller.SetValue(a.ctx, a.editorView.path, val)
	if err != nil {
		if store.IsAuthRequired(err) {
			return a, a.showLoginView("Session expired. Please log in again.")
		}
		a.statusMsg = "Set failed: " + err.Error()
	} else {
		a.statusMsg = fmt.Sprintf("Updated %s", a.editorView.path)
		a.refreshTreeView()
	}
	a.activeView = viewTree
	return a, nil
}

func (a *App) updateComponentView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			a.activeView = viewTree
			a.statusMsg = ""
			a.refreshTreeView()
			return a, nil
		}
	}

	var cmd tea.Cmd
	a.componentView, cmd = a.componentView.UpdateComponent(msg)
	if a.componentView.message != "" {
		a.statusMsg = a.componentView.message
	}
	return a, cmd
}

func (a *App) updateValidationView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			a.activeView = viewTree
			a.statusMsg = ""
			return a, nil
		}
	}

	var cmd tea.Cmd
	a.validationView, cmd = a.validationView.UpdateValidation(msg)
	return a, cmd
}

func (a *App) updateApplyView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	a.applyView, cmd = a.applyView.UpdateApply(msg)
	return a, cmd
}

func (a *App) updateMarketplaceView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			if a.marketplaceView.searching {
				// Let the marketplace view handle esc during search.
				break
			}
			a.activeView = viewTree
			a.statusMsg = ""
			return a, nil
		case "i":
			// Open detail view for selected entry.
			if entry, ok := a.marketplaceView.SelectedEntry(); ok {
				a.pluginDetailView = NewPluginDetailView(a.ctx, a.controller, entry.Type, entry.Publisher, entry.Name)
				a.pluginDetailView.SetSize(a.width, a.contentHeight())
				a.activeView = viewPluginDetail
				return a, a.pluginDetailView.Init()
			}
		}
	}

	var cmd tea.Cmd
	a.marketplaceView, cmd = a.marketplaceView.UpdateMarketplace(msg)
	return a, cmd
}

func (a *App) updateInstalledView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			a.activeView = viewTree
			a.statusMsg = ""
			return a, nil
		}
	}

	var cmd tea.Cmd
	a.installedView, cmd = a.installedView.UpdateInstalled(msg)
	return a, cmd
}

func (a *App) updatePluginDetailView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			a.activeView = viewMarketplace
			a.statusMsg = ""
			return a, nil
		}
	}

	var cmd tea.Cmd
	a.pluginDetailView, cmd = a.pluginDetailView.UpdateDetail(msg)
	return a, cmd
}

func (a *App) updateExportView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			a.activeView = viewTree
			a.statusMsg = ""
			return a, nil
		}
	}

	var cmd tea.Cmd
	a.exportView, cmd = a.exportView.UpdateExport(msg, a.ctx)
	return a, cmd
}

// View renders the current view.
func (a *App) View() string {
	header := a.renderHeader()
	content := a.renderContent()
	status := a.renderStatusBar()

	if a.showHelp {
		content = a.renderHelp()
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, content, status)
}

func (a *App) renderHeader() string {
	title := fmt.Sprintf(" zhi - %s ", a.controller.WorkspaceName())
	viewName := ""
	switch a.activeView {
	case viewLogin:
		viewName = "Login"
	case viewTree:
		viewName = "Tree"
	case viewEditor:
		viewName = "Editor"
	case viewComponent:
		viewName = "Components"
	case viewValidation:
		viewName = "Validation"
	case viewApply:
		viewName = "Apply"
	case viewExport:
		viewName = "Export"
	case viewMarketplace:
		viewName = "Marketplace"
	case viewInstalled:
		viewName = "Installed"
	case viewPluginDetail:
		viewName = "Plugin Detail"
	}

	left := HeaderStyle.Render(title)
	right := HeaderStyle.Render(fmt.Sprintf(" %s ", viewName))
	gap := max(a.width-lipgloss.Width(left)-lipgloss.Width(right), 0)
	mid := HeaderStyle.Render(fmt.Sprintf("%*s", gap, ""))
	return left + mid + right
}

func (a *App) renderContent() string {
	switch a.activeView {
	case viewLogin:
		return a.loginView.View()
	case viewTree:
		return a.treeView.View()
	case viewEditor:
		return a.editorView.View()
	case viewComponent:
		return a.componentView.View()
	case viewValidation:
		return a.validationView.View()
	case viewApply:
		return a.applyView.View()
	case viewExport:
		return a.exportView.View()
	case viewMarketplace:
		return a.marketplaceView.View()
	case viewInstalled:
		return a.installedView.View()
	case viewPluginDetail:
		return a.pluginDetailView.View()
	default:
		return ""
	}
}

func (a *App) renderStatusBar() string {
	var hints string
	switch a.activeView {
	case viewLogin:
		hints = "Tab:next field  Enter:submit  Ctrl+C:quit"
	case viewTree:
		hints = "j/k:navigate  enter:edit  s:save  v:validate  a:apply  e:export  c:components  m:marketplace  p:plugins  r:reload  ?:help  q:quit"
	case viewEditor:
		hints = "enter:save  esc:cancel"
	case viewComponent:
		hints = "j/k:navigate  enter/space:toggle  i:info  esc:back  ?:help"
	case viewValidation:
		hints = "j/k:scroll  esc:back"
	case viewApply:
		if a.applyView.running {
			hints = "j/k:scroll  G:bottom  g:top  ctrl+c:cancel"
		} else {
			hints = "j/k:scroll  G:bottom  g:top  esc/q:back"
		}
	case viewExport:
		hints = "j/k:navigate  enter:export  p:preview  esc:back"
	case viewMarketplace:
		hints = "j/k:navigate  /:search  enter:install  i:detail  t:type  o:sort  esc:back"
	case viewInstalled:
		hints = "j/k:navigate  u:update  d:uninstall  r:refresh  esc:back"
	case viewPluginDetail:
		hints = "j/k:scroll  enter:install  esc:back"
	}

	left := StatusBarStyle.Render(a.statusMsg)
	right := StatusBarStyle.Render(hints)
	gap := max(a.width-lipgloss.Width(left)-lipgloss.Width(right), 0)
	mid := StatusBarStyle.Render(fmt.Sprintf("%*s", gap, ""))
	return left + mid + right
}

func (a *App) renderHelp() string {
	help := ""
	switch a.activeView {
	case viewTree:
		help = lipgloss.JoinVertical(lipgloss.Left,
			HeaderStyle.Render(" Key Bindings - Tree View "),
			"",
			helpLine("j / ↓", "Move cursor down"),
			helpLine("k / ↑", "Move cursor up"),
			helpLine("Enter", "Open value editor"),
			helpLine("/", "Filter paths"),
			helpLine("s", "Save tree to store"),
			helpLine("v", "Validate and show results"),
			helpLine("a", "Run apply command"),
			helpLine("e", "Open export view"),
			helpLine("c", "Open component view"),
			helpLine("r", "Reload tree from provider"),
			helpLine("m", "Open marketplace"),
			helpLine("p", "View installed plugins"),
			helpLine("?", "Toggle this help"),
			helpLine("q", "Quit"),
			helpLine("Esc", "Clear filter / close help"),
		)
	case viewComponent:
		help = lipgloss.JoinVertical(lipgloss.Left,
			HeaderStyle.Render(" Key Bindings - Component View "),
			"",
			helpLine("j / ↓", "Move cursor down"),
			helpLine("k / ↑", "Move cursor up"),
			helpLine("Enter / Space", "Toggle component"),
			helpLine("i", "Show component info"),
			helpLine("?", "Toggle this help"),
			helpLine("Esc", "Return to tree view"),
		)
	}
	return help
}

func helpLine(key, desc string) string {
	return fmt.Sprintf("  %s  %s",
		HelpKeyStyle.Render(fmt.Sprintf("%-15s", key)),
		HelpDescStyle.Render(desc))
}

func (a *App) contentHeight() int {
	// Header (1) + status bar (1) = 2 lines of chrome
	h := max(a.height-2, 1)
	return h
}

func (a *App) refreshTreeView() {
	if a.tree != nil {
		a.treeView = NewTreeView(a.controller, a.tree)
		a.treeView.SetSize(a.width, a.contentHeight())
	}
}
