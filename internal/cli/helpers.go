package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// exitCoder is implemented by errors that carry a specific exit code.
type exitCoder interface {
	ExitCode() int
}

// ExitCode extracts an exit code from an error. Returns 0 if the error
// does not implement exitCoder.
func ExitCode(err error) int {
	var ec exitCoder
	if ok := errors.As(err, &ec); ok {
		return ec.ExitCode()
	}
	return 0
}

// stateFilePath returns the path to the component state file.
func stateFilePath(wsDir string) string {
	return filepath.Join(wsDir, ".zhi", "components.json")
}

// parseComponentState parses a component state JSON file.
func parseComponentState(data []byte) (map[string]bool, error) {
	var state map[string]bool
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return state, nil
}

// saveComponentState persists component state to the state file.
// The .zhi directory is created with 0700 (user-only) and state files
// are written with 0600 permissions.
func saveComponentState(wsDir string, state map[string]bool) error {
	path := stateFilePath(wsDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// padRight pads a string to the given width.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// fprintTable prints a simple text table to the given writer.
func fprintTable(w io.Writer, headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header.
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprint(w, padRight(h, widths[i]))
	}
	fmt.Fprintln(w)

	// Print rows.
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				fmt.Fprint(w, "  ")
			}
			if i < len(widths) {
				fmt.Fprint(w, padRight(cell, widths[i]))
			} else {
				fmt.Fprint(w, cell)
			}
		}
		fmt.Fprintln(w)
	}
}
