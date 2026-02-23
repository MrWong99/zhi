package core

import (
	"fmt"
	"os"
	"strings"
)

// DiffResult holds the result of comparing a rendered export with an existing file.
type DiffResult struct {
	// OutputPath is the file being compared.
	OutputPath string
	// HasChanges indicates whether the new content differs from the existing file.
	HasChanges bool
	// IsNew indicates the file does not exist yet.
	IsNew bool
	// Diff is the unified diff output.
	Diff string
}

// DiffExport compares the rendered content with the existing file at outputPath.
// If the file does not exist, it shows all lines as additions.
func DiffExport(outputPath, newContent string) (*DiffResult, error) {
	result := &DiffResult{
		OutputPath: outputPath,
	}

	existing, err := os.ReadFile(outputPath)
	if err != nil {
		if os.IsNotExist(err) {
			result.IsNew = true
			result.HasChanges = true
			result.Diff = formatNewFileDiff(outputPath, newContent)
			return result, nil
		}
		return nil, fmt.Errorf("reading existing file %s: %w", outputPath, err)
	}

	oldContent := string(existing)
	if oldContent == newContent {
		result.HasChanges = false
		return result, nil
	}

	result.HasChanges = true
	result.Diff = unifiedDiff(outputPath, oldContent, newContent)
	return result, nil
}

// formatNewFileDiff formats a diff for a file that doesn't exist yet.
func formatNewFileDiff(path, content string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "--- /dev/null\n")
	fmt.Fprintf(&sb, "+++ %s\n", path)

	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	fmt.Fprintf(&sb, "@@ -0,0 +1,%d @@\n", len(lines))
	for _, line := range lines {
		fmt.Fprintf(&sb, "+%s\n", line)
	}
	return sb.String()
}

// unifiedDiff produces a simple unified diff between old and new content.
// It uses a basic line-by-line comparison with context lines.
func unifiedDiff(path, oldContent, newContent string) string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Remove trailing empty line from split
	if len(oldLines) > 0 && oldLines[len(oldLines)-1] == "" {
		oldLines = oldLines[:len(oldLines)-1]
	}
	if len(newLines) > 0 && newLines[len(newLines)-1] == "" {
		newLines = newLines[:len(newLines)-1]
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "--- %s (current)\n", path)
	fmt.Fprintf(&sb, "+++ %s (new)\n", path)

	// Compute LCS-based diff using a simple O(mn) algorithm.
	// For config files this is fine; they're rarely huge.
	hunks := computeHunks(oldLines, newLines, 3)
	for _, h := range hunks {
		sb.WriteString(h)
	}
	return sb.String()
}

// computeHunks generates unified diff hunks with context lines.
func computeHunks(oldLines, newLines []string, contextLines int) []string {
	// Compute edit script using LCS.
	lcs := lcsMatrix(oldLines, newLines)
	edits := backtrack(lcs, oldLines, newLines)

	if len(edits) == 0 {
		return nil
	}

	// Group edits into hunks with context.
	type edit struct {
		op   byte // ' ', '-', '+'
		line string
		oldN int // 1-based line number in old
		newN int // 1-based line number in new
	}

	var allEdits []edit
	oi, ni := 0, 0
	for _, e := range edits {
		switch e.op {
		case '=':
			oi++
			ni++
			allEdits = append(allEdits, edit{' ', e.line, oi, ni})
		case '-':
			oi++
			allEdits = append(allEdits, edit{'-', e.line, oi, 0})
		case '+':
			ni++
			allEdits = append(allEdits, edit{'+', e.line, 0, ni})
		}
	}

	// Find changed regions and add context.
	var hunks []string
	i := 0
	for i < len(allEdits) {
		// Skip unchanged lines until we find a change.
		if allEdits[i].op == ' ' {
			i++
			continue
		}

		// Found a change. Determine hunk boundaries.
		start := i - contextLines
		if start < 0 {
			start = 0
		}

		// Find the end of this change group (including context).
		end := i
		for end < len(allEdits) {
			if allEdits[end].op != ' ' {
				end++
				continue
			}
			// Count consecutive context lines.
			ctxEnd := end
			for ctxEnd < len(allEdits) && allEdits[ctxEnd].op == ' ' {
				ctxEnd++
			}
			if ctxEnd-end > 2*contextLines || ctxEnd >= len(allEdits) {
				// Gap too large or end of file; close this hunk.
				end = end + contextLines
				if end > len(allEdits) {
					end = len(allEdits)
				}
				break
			}
			// Include intermediate context in this hunk.
			end = ctxEnd
		}

		// Compute hunk header.
		var oldStart, oldCount, newStart, newCount int
		for j := start; j < end; j++ {
			switch allEdits[j].op {
			case ' ':
				if oldStart == 0 {
					oldStart = allEdits[j].oldN
				}
				if newStart == 0 {
					newStart = allEdits[j].newN
				}
				oldCount++
				newCount++
			case '-':
				if oldStart == 0 {
					oldStart = allEdits[j].oldN
				}
				if newStart == 0 {
					// Use next new line number.
					for k := j + 1; k < end; k++ {
						if allEdits[k].newN > 0 {
							newStart = allEdits[k].newN
							break
						}
					}
					if newStart == 0 {
						newStart = 1
					}
				}
				oldCount++
			case '+':
				if newStart == 0 {
					newStart = allEdits[j].newN
				}
				if oldStart == 0 {
					for k := j + 1; k < end; k++ {
						if allEdits[k].oldN > 0 {
							oldStart = allEdits[k].oldN
							break
						}
					}
					if oldStart == 0 {
						oldStart = 1
					}
				}
				newCount++
			}
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
		for j := start; j < end; j++ {
			fmt.Fprintf(&sb, "%c%s\n", allEdits[j].op, allEdits[j].line)
		}
		hunks = append(hunks, sb.String())
		i = end
	}
	return hunks
}

type editOp struct {
	op   byte // '=', '-', '+'
	line string
}

// lcsMatrix computes the LCS table for two slices of strings.
func lcsMatrix(a, b []string) [][]int {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}
	return dp
}

// backtrack produces a sequence of edit operations from the LCS matrix.
func backtrack(dp [][]int, a, b []string) []editOp {
	var ops []editOp
	i, j := len(a), len(b)
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && a[i-1] == b[j-1] {
			ops = append([]editOp{{op: '=', line: a[i-1]}}, ops...)
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			ops = append([]editOp{{op: '+', line: b[j-1]}}, ops...)
			j--
		} else {
			ops = append([]editOp{{op: '-', line: a[i-1]}}, ops...)
			i--
		}
	}
	return ops
}
