package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"relaydesk/api/internal/domain"
)

type Classification struct {
	Title      string          `json:"title"`
	Summary    string          `json:"summary"`
	Category   string          `json:"category"`
	Priority   domain.Priority `json:"priority"`
	Confidence float64         `json:"confidence"`
	Provider   string          `json:"provider"`
	Tags       []string        `json:"tags"`
}

type Classifier interface {
	Classify(context.Context, string) Classification
}

type HTTPClassifier struct {
	url    string
	client *http.Client
}

func NewHTTPClassifier(url string) *HTTPClassifier {
	return &HTTPClassifier{url: strings.TrimRight(url, "/"), client: &http.Client{Timeout: 2 * time.Second}}
}

func (c *HTTPClassifier) Classify(ctx context.Context, body string) Classification {
	payload, _ := json.Marshal(map[string]string{"text": body})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/classify", bytes.NewReader(payload))
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		if res, requestErr := c.client.Do(req); requestErr == nil {
			defer res.Body.Close()
			var result Classification
			if res.StatusCode == http.StatusOK && json.NewDecoder(res.Body).Decode(&result) == nil {
				result.Provider = "local-ml"
				return result
			}
		}
	}
	return fallbackClassification(body)
}

func fallbackClassification(body string) Classification {
	lower := strings.ToLower(body)
	result := Classification{
		Title:      truncate(body, 64),
		Summary:    truncate(body, 148),
		Category:   "general",
		Priority:   domain.PriorityNormal,
		Confidence: 0.62,
		Provider:   "rule-fallback",
		Tags:       []string{"auto-triaged"},
	}
	if containsAny(lower, "outage", "down", "blocked", "production", "urgent") {
		result.Category, result.Priority, result.Confidence = "incident", domain.PriorityUrgent, 0.91
		result.Tags = append(result.Tags, "incident")
	} else if containsAny(lower, "invoice", "billing", "charged", "payment") {
		result.Category, result.Priority, result.Confidence = "billing", domain.PriorityHigh, 0.86
		result.Tags = append(result.Tags, "billing")
	} else if containsAny(lower, "login", "sso", "permission", "access") {
		result.Category, result.Priority, result.Confidence = "access", domain.PriorityHigh, 0.84
		result.Tags = append(result.Tags, "access")
	} else if containsAny(lower, "feature", "could you", "request") {
		result.Category, result.Priority, result.Confidence = "feature", domain.PriorityLow, 0.78
		result.Tags = append(result.Tags, "feature-request")
	}
	return result
}

type JevClassifier struct {
	apiKey   string
	model    string
	endpoint string
	client   *http.Client
	fallback Classifier
}

func NewJevClassifier(apiKey, model string, fallback Classifier) Classifier {
	if strings.TrimSpace(apiKey) == "" {
		return fallback
	}
	if model == "" {
		model = "jev-latest"
	}
	return &JevClassifier{apiKey: apiKey, model: model, endpoint: "https://api.typesafe.ai/v1/systemone", fallback: fallback, client: &http.Client{Timeout: 4 * time.Second}}
}

type jevAnswer struct {
	Choice        string             `json:"choice"`
	Confidence    float64            `json:"confidence"`
	Noul          float64            `json:"noul"`
	Probabilities map[string]float64 `json:"probabilities"`
}

func (c *JevClassifier) Classify(ctx context.Context, body string) Classification {
	requestBody := map[string]any{
		"model": c.model,
		"state": map[string]string{"customer_message": body},
		"questions": map[string]any{
			"category": map[string]any{
				"type":         "choice",
				"instructions": "Route this customer support request to exactly one category.",
				"criteria": map[string]string{
					"incident": "An outage, error, broken behavior, or production-impacting technical problem",
					"billing":  "Invoices, payments, charges, refunds, or subscription billing",
					"access":   "Login, SSO, account access, invitations, roles, or permissions",
					"feature":  "A request for new or changed product behavior",
					"question": "A how-to, documentation, plan, or general product question",
				},
			},
			"priority": map[string]any{
				"type":         "choice",
				"instructions": "Choose support priority based on user impact and time sensitivity.",
				"criteria": map[string]string{
					"urgent": "Production outage, security risk, data loss, or many users blocked",
					"high":   "Important workflow blocked or multiple users affected, but not a broad outage",
					"normal": "A routine problem or question with a reasonable workaround",
					"low":    "Feedback or a feature request with no immediate impact",
				},
			},
			"human_review": map[string]any{
				"type":         "noul",
				"instructions": "Should a human support agent review this request promptly?",
				"criteria":     map[string]string{"true": "Human judgment or action is needed", "false": "Safe to handle automatically"},
			},
		},
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return c.fallback.Classify(ctx, body)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return c.fallback.Classify(ctx, body)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.client.Do(req)
	if err != nil {
		return c.fallback.Classify(ctx, body)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return c.fallback.Classify(ctx, body)
	}
	var response struct {
		Answers map[string]jevAnswer `json:"answers"`
	}
	if json.NewDecoder(res.Body).Decode(&response) != nil {
		return c.fallback.Classify(ctx, body)
	}
	category, categoryOK := response.Answers["category"]
	priority, priorityOK := response.Answers["priority"]
	if !categoryOK || !priorityOK || !validCategory(category.Choice) || !validPriority(priority.Choice) {
		return c.fallback.Classify(ctx, body)
	}
	tags := []string{category.Choice, "jev-triaged"}
	if response.Answers["human_review"].Noul >= 0.65 {
		tags = append(tags, "human-review")
	}
	confidence := category.Confidence
	if priority.Confidence < confidence {
		confidence = priority.Confidence
	}
	return Classification{
		Title: truncate(body, 64), Summary: truncate(body, 148), Category: category.Choice,
		Priority: domain.Priority(priority.Choice), Confidence: confidence, Provider: "jev", Tags: tags,
	}
}

func validCategory(value string) bool {
	switch value {
	case "incident", "billing", "access", "feature", "question":
		return true
	}
	return false
}

func validPriority(value string) bool {
	switch domain.Priority(value) {
	case domain.PriorityUrgent, domain.PriorityHigh, domain.PriorityNormal, domain.PriorityLow:
		return true
	}
	return false
}

func containsAny(value string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return strings.TrimSpace(value[:limit-1]) + "…"
}
