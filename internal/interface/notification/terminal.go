package notification

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

// TerminalNotifier sends notifications to the terminal.
type TerminalNotifier struct {
	colorEnabled bool
	writer       *os.File
}

// NewTerminalNotifier creates a new terminal notifier.
func NewTerminalNotifier() *TerminalNotifier {
	return &TerminalNotifier{
		colorEnabled: isTerminal(),
		writer:       os.Stderr,
	}
}

// Name returns the notifier name.
func (t *TerminalNotifier) Name() string {
	return "terminal"
}

// SupportsResponse returns whether this notifier supports responses.
func (t *TerminalNotifier) SupportsResponse() bool {
	return false // Terminal is output-only in non-interactive mode
}

// Send sends a notification to the terminal.
func (t *TerminalNotifier) Send(ctx context.Context, n *Notification) error {
	var sb strings.Builder

	// Add separator
	sb.WriteString("\n")
	sb.WriteString(t.color("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━", colorDim))
	sb.WriteString("\n")

	// Priority indicator
	icon := t.getPriorityIcon(n.Priority)
	sb.WriteString(icon)
	sb.WriteString(" ")

	// Title with priority color
	titleColor := t.getPriorityColor(n.Priority)
	sb.WriteString(t.color(n.Title, titleColor))
	sb.WriteString("\n")

	// Category and timestamp
	sb.WriteString(t.color(fmt.Sprintf("[%s] %s", n.Category, n.CreatedAt.Format(time.RFC3339)), colorDim))
	sb.WriteString("\n\n")

	// Message
	sb.WriteString(n.Message)
	sb.WriteString("\n")

	// Details
	if len(n.Details) > 0 {
		sb.WriteString("\n")
		sb.WriteString(t.color("Details:", colorBold))
		sb.WriteString("\n")
		for key, value := range n.Details {
			sb.WriteString(fmt.Sprintf("  %s: %v\n", key, value))
		}
	}

	// Actions
	if len(n.Actions) > 0 {
		sb.WriteString("\n")
		sb.WriteString(t.color("Available Actions:", colorBold))
		sb.WriteString("\n")
		for _, action := range n.Actions {
			marker := "○"
			if action.IsDefault {
				marker = "●"
			}
			if action.IsDangerous {
				sb.WriteString(fmt.Sprintf("  %s %s - %s %s\n",
					marker,
					t.color(action.Label, colorRed),
					action.Description,
					t.color("(dangerous)", colorRed),
				))
			} else {
				sb.WriteString(fmt.Sprintf("  %s %s - %s\n", marker, action.Label, action.Description))
			}
		}
		sb.WriteString(fmt.Sprintf("\n  Respond with: agent-collab notify respond %s <action>\n", n.ID))
	}

	sb.WriteString(t.color("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━", colorDim))
	sb.WriteString("\n\n")

	_, err := t.writer.WriteString(sb.String())
	return err
}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
)

func (t *TerminalNotifier) color(text, color string) string {
	if !t.colorEnabled {
		return text
	}
	return color + text + colorReset
}

func (t *TerminalNotifier) getPriorityIcon(p Priority) string {
	switch p {
	case PriorityLow:
		return t.color("ℹ", colorBlue)
	case PriorityNormal:
		return t.color("●", colorGreen)
	case PriorityHigh:
		return t.color("⚠", colorYellow)
	case PriorityCritical:
		return t.color("🚨", colorRed)
	default:
		return "●"
	}
}

func (t *TerminalNotifier) getPriorityColor(p Priority) string {
	switch p {
	case PriorityLow:
		return colorBlue
	case PriorityNormal:
		return colorGreen
	case PriorityHigh:
		return colorYellow
	case PriorityCritical:
		return colorRed
	default:
		return colorReset
	}
}

// isTerminal checks if stderr is a terminal.
func isTerminal() bool {
	fileInfo, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
