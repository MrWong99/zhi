package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/internal/ui"
)

// ApplyOutputMsg delivers a batch of apply output lines.
type ApplyOutputMsg struct {
	Lines []core.ApplyOutput
}

// ApplyDoneMsg signals apply completion.
type ApplyDoneMsg struct {
	Result *core.ApplyResult
	Err    error
}

// ApplyView streams apply command output in a scrollable viewport.
type ApplyView struct {
	output   []core.ApplyOutput
	lines    []string
	running  bool
	result   *core.ApplyResult
	err      error
	spinner  spinner.Model
	cursor   int
	offset   int
	follow   bool
	cancelFn context.CancelFunc
	width    int
	height   int
	outputCh <-chan core.ApplyOutput // persisted for chained reads
	resultCh <-chan ApplyDoneMsg     // carries the apply result
}

// NewApplyView creates a new apply view.
func NewApplyView() ApplyView {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = SpinnerStyle

	return ApplyView{
		spinner: s,
		follow:  true,
	}
}

// SetSize updates the view dimensions.
func (v *ApplyView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// Init returns the initial command for the apply view.
func (v ApplyView) Init() tea.Cmd {
	return v.spinner.Tick
}

// StartApply begins the apply command execution.
func (v *ApplyView) StartApply(ctx context.Context, controller *ui.UIController) tea.Cmd {
	v.running = true
	v.output = nil
	v.lines = nil
	v.result = nil
	v.err = nil
	v.follow = true

	applyCtx, cancel := context.WithCancel(ctx)
	v.cancelFn = cancel

	outputCh := make(chan core.ApplyOutput, 100)
	resultCh := make(chan ApplyDoneMsg, 1)

	v.outputCh = outputCh
	v.resultCh = resultCh

	// Start the apply goroutine.
	go func() {
		result, err := controller.Apply(applyCtx, "", outputCh)
		resultCh <- ApplyDoneMsg{Result: result, Err: err}
	}()

	return tea.Batch(
		v.spinner.Tick,
		v.readOutput(),
	)
}

func (v *ApplyView) readOutput() tea.Cmd {
	ch := v.outputCh
	rch := v.resultCh
	return func() tea.Msg {
		var batch []core.ApplyOutput
		timer := time.NewTimer(50 * time.Millisecond)
		defer timer.Stop()

		for {
			select {
			case line, ok := <-ch:
				if !ok {
					// Channel closed = command finished.
					if len(batch) > 0 {
						return ApplyOutputMsg{Lines: batch}
					}
					return <-rch
				}
				batch = append(batch, line)
				// If we've accumulated enough, send the batch.
				if len(batch) >= 50 {
					return ApplyOutputMsg{Lines: batch}
				}
			case <-timer.C:
				if len(batch) > 0 {
					return ApplyOutputMsg{Lines: batch}
				}
				// Reset timer and continue waiting.
				timer.Reset(50 * time.Millisecond)
			case res := <-rch:
				// Defensive: the result arrived even though the output
				// channel was not closed (e.g. a controller that forgot
				// to close it). Flush any pending output first, then
				// finish so the view can never wedge waiting on a close.
				if len(batch) > 0 {
					return ApplyOutputMsg{Lines: batch}
				}
				return res
			}
		}
	}
}

// UpdateApply handles messages for the apply view.
func (v ApplyView) UpdateApply(msg tea.Msg) (ApplyView, tea.Cmd) {
	switch msg := msg.(type) {
	case ApplyOutputMsg:
		v.output = append(v.output, msg.Lines...)
		for _, line := range msg.Lines {
			prefix := ""
			if line.Stream == "stderr" {
				prefix = ErrorStyle.Render("[stderr] ")
			}
			v.lines = append(v.lines, prefix+line.Line)
		}
		if v.follow {
			v.cursor = len(v.lines) - 1
			v.ensureVisible()
		}
		return v, v.readOutput()

	case ApplyDoneMsg:
		v.running = false
		v.result = msg.Result
		v.err = msg.Err
		if msg.Err != nil {
			v.lines = append(v.lines, "")
			v.lines = append(v.lines, ErrorStyle.Render(fmt.Sprintf("Apply error: %s", msg.Err)))
		} else if msg.Result != nil {
			v.lines = append(v.lines, "")
			if msg.Result.ExitCode == 0 {
				v.lines = append(v.lines, SuccessStyle.Render(fmt.Sprintf("Apply completed (exit code %d, duration %s)", msg.Result.ExitCode, msg.Result.Duration.Round(time.Millisecond))))
			} else {
				v.lines = append(v.lines, ErrorStyle.Render(fmt.Sprintf("Apply failed (exit code %d, duration %s)", msg.Result.ExitCode, msg.Result.Duration.Round(time.Millisecond))))
			}
		} else {
			v.lines = append(v.lines, "")
			v.lines = append(v.lines, DimStyle.Render("Apply completed."))
		}
		if v.follow {
			v.cursor = len(v.lines) - 1
			v.ensureVisible()
		}
		return v, nil

	case spinner.TickMsg:
		if v.running {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return v, cmd
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			v.follow = false
			if v.cursor < len(v.lines)-1 {
				v.cursor++
				v.ensureVisible()
			}
		case "k", "up":
			v.follow = false
			if v.cursor > 0 {
				v.cursor--
				v.ensureVisible()
			}
		case "G":
			v.follow = true
			v.cursor = len(v.lines) - 1
			v.ensureVisible()
		case "g":
			v.follow = false
			v.cursor = 0
			v.offset = 0
		case "ctrl+c":
			if v.running && v.cancelFn != nil {
				v.cancelFn()
				v.lines = append(v.lines, "")
				v.lines = append(v.lines, WarningStyle.Render("Cancelling..."))
			}
		}
	}

	return v, nil
}

func (v *ApplyView) ensureVisible() {
	if v.cursor < v.offset {
		v.offset = v.cursor
	}
	if v.cursor >= v.offset+v.height {
		v.offset = v.cursor - v.height + 1
	}
}

// View renders the apply view.
func (v ApplyView) View() string {
	var sb strings.Builder

	// Header with status.
	if v.running {
		fmt.Fprintf(&sb, "  %s Running apply command...\n\n", v.spinner.View())
	} else {
		sb.WriteString("  Apply Output:\n\n")
	}

	if len(v.lines) == 0 {
		sb.WriteString(DimStyle.Render("  Waiting for output..."))
		sb.WriteString("\n")
		return v.padHeight(sb.String())
	}

	visibleLines := max(
		// Account for header
		v.height-2, 1)

	end := min(v.offset+visibleLines, len(v.lines))

	for i := v.offset; i < end; i++ {
		line := v.lines[i]
		if i == v.cursor {
			sb.WriteString(ActiveStyle.Render("> " + line))
		} else {
			sb.WriteString("  " + line)
		}
		sb.WriteString("\n")
	}

	return v.padHeight(sb.String())
}

func (v ApplyView) padHeight(content string) string {
	lines := strings.Count(content, "\n")
	for lines < v.height {
		content += "\n"
		lines++
	}
	return content
}
