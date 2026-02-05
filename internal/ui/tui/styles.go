package tui

import "github.com/charmbracelet/lipgloss"

// Styles defines the visual theme for the TUI.
var (
	// HeaderStyle is used for the header bar at the top.
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	// PathStyle is used for config path display.
	PathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12"))

	// ValueStyle is used for config value display.
	ValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15"))

	// BlockingStyle is used for blocking validation issues.
	BlockingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true)

	// WarningStyle is used for warning validation issues.
	WarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11"))

	// InfoStyle is used for informational messages.
	InfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14"))

	// StatusBarStyle is used for the bottom status bar.
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	// ActiveStyle is used for the currently selected/highlighted item.
	ActiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("240")).
			Bold(true)

	// DisabledComponentStyle is used for paths belonging to disabled components.
	DisabledComponentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("8")).
				Faint(true)

	// ComponentBadgeStyle is used for component name badges.
	ComponentBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("13")).
				Bold(true)

	// MandatoryBadgeStyle is used for mandatory component indicators.
	MandatoryBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("9")).
				Bold(true)

	// CheckboxEnabledStyle is used for enabled component checkmarks.
	CheckboxEnabledStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("10")).
				Bold(true)

	// CheckboxDisabledStyle is used for disabled component empty boxes.
	CheckboxDisabledStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("8"))

	// ErrorStyle is used for error messages.
	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9"))

	// SuccessStyle is used for success messages.
	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10"))

	// DimStyle is used for secondary/dimmed text.
	DimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	// ComponentErrorStyle is used for component dependency errors.
	ComponentErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("5")).
				Bold(true)

	// HelpKeyStyle is used for keybinding keys in help text.
	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Bold(true)

	// HelpDescStyle is used for keybinding descriptions in help text.
	HelpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))

	// SpinnerStyle is used for the activity spinner.
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205"))

	// PasswordStyle is used for masked password values.
	PasswordStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	// ReadonlyBadgeStyle is used for the [readonly] badge.
	ReadonlyBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("8")).
				Italic(true)

	// RequiredBadgeStyle is used for the [required] badge.
	RequiredBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("9"))

	// DeprecatedStyle is used for deprecated value paths.
	DeprecatedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Strikethrough(true)

	// SectionHeaderStyle is used for section headings in the tree view.
	SectionHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("14")).
				Bold(true).
				Underline(true)

	// GroupBadgeStyle is used for the group badge in the tree view.
	GroupBadgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6"))

	// ConfirmStyle is used for the [confirm] badge.
	ConfirmStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11"))

	// EnumBadgeStyle is used for the enum indicator badge.
	EnumBadgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Italic(true)
)
