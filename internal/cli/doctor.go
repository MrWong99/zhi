package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core/doctor"
)

var (
	doctorChecks  []string
	doctorJSON    bool
	doctorQuiet   bool
	doctorFix     bool
	doctorDeep    bool
	doctorTimeout time.Duration
	doctorNoColor bool
	doctorUpdates bool
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose workspace health",
	Long: `Run a battery of health checks against the current workspace.

Checks are grouped into categories: workspace (integrity of zhi.yaml),
plugins (referenced plugins installed and compatible), store (store
plugin reachability, including Vault seal/token probes when vault is
configured), config (tree validation, deprecated fields), and updates
(marketplace version comparison — opt-in).

The exit code is 1 if any check fails, 0 otherwise. Warnings do not
affect the exit code.`,
	Example: `  zhi doctor
  zhi doctor --check workspace --check plugins
  zhi doctor --check updates
  zhi doctor --json | jq .
  zhi doctor --deep`,
	RunE: runDoctor,
}

func init() {
	f := doctorCmd.Flags()
	f.StringSliceVar(&doctorChecks, "check", nil,
		"run only the given categories (workspace|plugins|store|config|updates)")
	f.BoolVar(&doctorJSON, "json", false, "machine-readable output")
	f.BoolVar(&doctorQuiet, "quiet", false, "suppress OK and skipped results in text output")
	f.BoolVar(&doctorFix, "fix", false, "apply auto-fixable remediations (empty set in v1)")
	f.BoolVar(&doctorDeep, "deep", false, "enable slow / expensive checks (signature verification, etc.)")
	f.DurationVar(&doctorTimeout, "timeout", 10*time.Second, "per-check timeout")
	f.BoolVar(&doctorNoColor, "no-color", false, "disable colored output")
	f.BoolVar(&doctorUpdates, "updates", false,
		"shortcut: include the updates category (same as --check updates)")
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	applyVerbose()

	// If --updates is set, ensure "updates" is in the list.
	checkFlags := slices.Clone(doctorChecks)
	if doctorUpdates && !slices.Contains(checkFlags, string(doctor.CategoryUpdates)) {
		checkFlags = append(checkFlags, string(doctor.CategoryUpdates))
	}

	cats, err := doctor.ParseCategories(checkFlags)
	if err != nil {
		return err
	}

	env := doctor.BuildEnvironment(doctor.EnvironmentOptions{
		WorkspacePath: workspace,
		ZhiVersion:    appVersion,
		Deep:          doctorDeep,
		Timeout:       doctorTimeout,
		Updates:       slices.Contains(cats, doctor.CategoryUpdates),
	})
	defer env.Close()

	runner := doctor.NewRunner(doctor.AllChecks(), cats)
	report := runner.Run(cmd.Context(), env)

	w := cmd.OutOrStdout()
	if doctorJSON {
		if err := writeJSON(w, report); err != nil {
			return err
		}
	} else {
		writeText(w, report, doctorQuiet, useColor(w))
	}

	if doctorFix {
		fmt.Fprintln(w, "\n--fix has no registered fixers in this release.")
	}

	if report.HasError() {
		return &doctorExitError{}
	}
	return nil
}

// doctorExitError carries a non-zero exit code without printing anything
// beyond what was already written.
type doctorExitError struct{}

func (doctorExitError) Error() string { return "doctor reported errors" }
func (doctorExitError) ExitCode() int { return 1 }

// writeJSON serialises the report as a single object with grouped
// results plus a summary stanza so callers can branch on counts.
func writeJSON(w io.Writer, r doctor.Report) error {
	ok, warn, errCount, skipped := r.Counts()
	payload := struct {
		Groups  []doctor.CategoryGroup `json:"groups"`
		Summary struct {
			OK       int `json:"ok"`
			Warnings int `json:"warnings"`
			Errors   int `json:"errors"`
			Skipped  int `json:"skipped"`
			Total    int `json:"total"`
			ExitCode int `json:"exit_code"`
		} `json:"summary"`
	}{Groups: r.Groups}
	payload.Summary.OK = ok
	payload.Summary.Warnings = warn
	payload.Summary.Errors = errCount
	payload.Summary.Skipped = skipped
	payload.Summary.Total = r.Total()
	payload.Summary.ExitCode = r.ExitCode()

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

// writeText renders the report in the human-friendly table-ish format
// shown in the command's example output. Quiet mode suppresses OK and
// Skipped lines, keeping only Warnings and Errors.
func writeText(w io.Writer, r doctor.Report, quiet, color bool) {
	fmt.Fprintln(w, "Running zhi doctor…")
	fmt.Fprintln(w)

	for _, g := range r.Groups {
		header := statusLabel(groupStatus(g), color) + "  " +
			titleCase(string(g.Category)) + summary(g)
		fmt.Fprintln(w, header)

		for _, res := range g.Results {
			if quiet && (res.Status == doctor.StatusOK || res.Status == doctor.StatusSkipped) {
				continue
			}
			fmt.Fprint(w, "  ", statusIcon(res.Status, color), " ", res.Name)
			if res.Message != "" {
				fmt.Fprint(w, " — ", res.Message)
			}
			fmt.Fprintln(w)
			if res.Detail != "" {
				for _, line := range strings.Split(strings.TrimRight(res.Detail, "\n"), "\n") {
					fmt.Fprintln(w, "      ", line)
				}
			}
			if res.Hint != "" {
				fmt.Fprintln(w, "       hint:", res.Hint)
			}
		}
		fmt.Fprintln(w)
	}

	ok, warn, errs, skipped := r.Counts()
	total := ok + warn + errs
	fmt.Fprintf(w, "Summary: %d/%d checks passed, %d warning(s), %d error(s)",
		ok, total, warn, errs)
	if skipped > 0 {
		fmt.Fprintf(w, ", %d skipped", skipped)
	}
	fmt.Fprintln(w)
}

// groupStatus returns the most severe status in a category for the
// header label. Precedence from most to least severe: Error, Warning,
// OK, Skipped. OK outranks Skipped so a category with any successful
// check doesn't look like it was entirely skipped.
func groupStatus(g doctor.CategoryGroup) doctor.Status {
	var hasWarning, hasOK, hasSkipped bool
	for _, r := range g.Results {
		switch r.Status {
		case doctor.StatusError:
			return doctor.StatusError
		case doctor.StatusWarning:
			hasWarning = true
		case doctor.StatusOK:
			hasOK = true
		case doctor.StatusSkipped:
			hasSkipped = true
		}
	}
	switch {
	case hasWarning:
		return doctor.StatusWarning
	case hasOK:
		return doctor.StatusOK
	case hasSkipped:
		return doctor.StatusSkipped
	}
	return doctor.StatusOK
}

// summary formats the per-category tally shown in the header.
func summary(g doctor.CategoryGroup) string {
	ok, warn, errs, skipped := 0, 0, 0, 0
	for _, r := range g.Results {
		switch r.Status {
		case doctor.StatusOK:
			ok++
		case doctor.StatusWarning:
			warn++
		case doctor.StatusError:
			errs++
		case doctor.StatusSkipped:
			skipped++
		}
	}
	total := ok + warn + errs
	var extras []string
	if warn > 0 {
		extras = append(extras, fmt.Sprintf("%d warning", warn))
	}
	if errs > 0 {
		extras = append(extras, fmt.Sprintf("%d error", errs))
	}
	if skipped > 0 {
		extras = append(extras, fmt.Sprintf("%d skipped", skipped))
	}
	s := fmt.Sprintf(" — %d/%d checks passed", ok, total)
	if len(extras) > 0 {
		s += " (" + strings.Join(extras, ", ") + ")"
	}
	return s
}

// statusIcon returns a short marker for per-result lines. Icons fall
// back to ASCII when color is disabled.
func statusIcon(s doctor.Status, color bool) string {
	switch s {
	case doctor.StatusOK:
		if color {
			return colorize("✓", ansiGreen)
		}
		return "[OK]"
	case doctor.StatusWarning:
		if color {
			return colorize("⚠", ansiYellow)
		}
		return "[WARN]"
	case doctor.StatusError:
		if color {
			return colorize("✗", ansiRed)
		}
		return "[FAIL]"
	case doctor.StatusSkipped:
		if color {
			return colorize("·", ansiGrey)
		}
		return "[SKIP]"
	}
	return "?"
}

// statusLabel returns the per-category heading prefix.
func statusLabel(s doctor.Status, color bool) string {
	switch s {
	case doctor.StatusOK:
		if color {
			return colorize("✅", "")
		}
		return "[OK] "
	case doctor.StatusWarning:
		if color {
			return colorize("⚠️ ", "")
		}
		return "[WARN]"
	case doctor.StatusError:
		if color {
			return colorize("❌", "")
		}
		return "[FAIL]"
	case doctor.StatusSkipped:
		if color {
			return colorize("·", ansiGrey)
		}
		return "[SKIP]"
	}
	return "?"
}

// titleCase capitalises the first letter of a category name for display.
// Lower-cased input is assumed.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// useColor decides whether to emit ANSI codes: respect --no-color and
// NO_COLOR, and fall back to ASCII when stdout is not a TTY.
func useColor(w io.Writer) bool {
	if doctorNoColor {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

const (
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
	ansiGrey   = "\x1b[90m"
	ansiReset  = "\x1b[0m"
)

func colorize(s, code string) string {
	if code == "" {
		return s
	}
	return code + s + ansiReset
}
