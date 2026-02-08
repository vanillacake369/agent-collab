package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookNotifier sends notifications via HTTP webhooks.
type WebhookNotifier struct {
	url     string
	client  *http.Client
	headers map[string]string
}

// WebhookConfig configures the webhook notifier.
type WebhookConfig struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Timeout time.Duration     `json:"timeout,omitempty"`
}

// NewWebhookNotifier creates a new webhook notifier.
func NewWebhookNotifier(cfg *WebhookConfig) *WebhookNotifier {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	return &WebhookNotifier{
		url: cfg.URL,
		client: &http.Client{
			Timeout: timeout,
		},
		headers: cfg.Headers,
	}
}

// Name returns the notifier name.
func (w *WebhookNotifier) Name() string {
	return "webhook"
}

// SupportsResponse returns whether this notifier supports responses.
func (w *WebhookNotifier) SupportsResponse() bool {
	return false // Responses come through a separate callback mechanism
}

// WebhookPayload is the payload sent to the webhook.
type WebhookPayload struct {
	ID        string         `json:"id"`
	Category  string         `json:"category"`
	Priority  string         `json:"priority"`
	Title     string         `json:"title"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	Actions   []ActionInfo   `json:"actions,omitempty"`
	Timestamp string         `json:"timestamp"`
	ExpiresAt string         `json:"expires_at,omitempty"`
}

// ActionInfo is action information for the webhook payload.
type ActionInfo struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// Send sends a notification via webhook.
func (w *WebhookNotifier) Send(ctx context.Context, n *Notification) error {
	payload := WebhookPayload{
		ID:        n.ID,
		Category:  string(n.Category),
		Priority:  n.Priority.String(),
		Title:     n.Title,
		Message:   n.Message,
		Details:   n.Details,
		Timestamp: n.CreatedAt.Format(time.RFC3339),
	}

	if !n.ExpiresAt.IsZero() {
		payload.ExpiresAt = n.ExpiresAt.Format(time.RFC3339)
	}

	for _, action := range n.Actions {
		payload.Actions = append(payload.Actions, ActionInfo{
			ID:          action.ID,
			Label:       action.Label,
			Description: action.Description,
		})
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", w.url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range w.headers {
		req.Header.Set(key, value)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned error status: %d", resp.StatusCode)
	}

	return nil
}

// SlackNotifier sends notifications to Slack.
type SlackNotifier struct {
	webhookURL string
	client     *http.Client
	channel    string
}

// NewSlackNotifier creates a new Slack notifier.
func NewSlackNotifier(webhookURL, channel string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		channel: channel,
	}
}

// Name returns the notifier name.
func (s *SlackNotifier) Name() string {
	return "slack"
}

// SupportsResponse returns whether this notifier supports responses.
func (s *SlackNotifier) SupportsResponse() bool {
	return false
}

// Send sends a notification to Slack.
func (s *SlackNotifier) Send(ctx context.Context, n *Notification) error {
	// Build Slack message blocks
	payload := map[string]any{
		"channel": s.channel,
		"blocks": []map[string]any{
			{
				"type": "header",
				"text": map[string]any{
					"type": "plain_text",
					"text": s.getPriorityEmoji(n.Priority) + " " + n.Title,
				},
			},
			{
				"type": "section",
				"text": map[string]any{
					"type": "mrkdwn",
					"text": n.Message,
				},
			},
		},
	}

	// Add details if present
	if len(n.Details) > 0 {
		fields := make([]map[string]any, 0)
		for key, value := range n.Details {
			fields = append(fields, map[string]any{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*%s:*\n%v", key, value),
			})
		}
		blocks := payload["blocks"].([]map[string]any)
		blocks = append(blocks, map[string]any{
			"type":   "section",
			"fields": fields,
		})
		payload["blocks"] = blocks
	}

	// Add actions if present
	if len(n.Actions) > 0 {
		elements := make([]map[string]any, 0)
		for _, action := range n.Actions {
			style := "primary"
			if action.IsDangerous {
				style = "danger"
			}
			elements = append(elements, map[string]any{
				"type":      "button",
				"text":      map[string]any{"type": "plain_text", "text": action.Label},
				"action_id": action.ID,
				"style":     style,
			})
		}
		blocks := payload["blocks"].([]map[string]any)
		blocks = append(blocks, map[string]any{
			"type":     "actions",
			"elements": elements,
		})
		payload["blocks"] = blocks
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.webhookURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create Slack request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("Slack request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Slack returned error status: %d", resp.StatusCode)
	}

	return nil
}

func (s *SlackNotifier) getPriorityEmoji(p Priority) string {
	switch p {
	case PriorityLow:
		return "ℹ️"
	case PriorityNormal:
		return "✅"
	case PriorityHigh:
		return "⚠️"
	case PriorityCritical:
		return "🚨"
	default:
		return "📢"
	}
}
