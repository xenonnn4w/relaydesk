package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"relaydesk/api/internal/domain"
)

type fixedClassifier struct{ result Classification }

func (f fixedClassifier) Classify(context.Context, string) Classification { return f.result }

func TestJevClassifierMapsTypedDecisions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"jev-test","answers":{"category":{"type":"choice","choice":"incident","confidence":0.96,"probabilities":{"incident":0.96}},"priority":{"type":"choice","choice":"urgent","confidence":0.91,"probabilities":{"urgent":0.91}},"human_review":{"type":"noul","noul":0.84}},"usage":{"input_tokens":20,"output_tokens":3}}`)
	}))
	defer server.Close()

	classifier := &JevClassifier{apiKey: "secret", model: "jev-latest", endpoint: server.URL, client: server.Client(), fallback: fixedClassifier{}}
	result := classifier.Classify(context.Background(), "Production checkout is down")
	if result.Provider != "jev" || result.Category != "incident" || result.Priority != domain.PriorityUrgent {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Confidence != 0.91 {
		t.Fatalf("confidence = %v, want 0.91", result.Confidence)
	}
	if got := result.Tags[len(result.Tags)-1]; got != "human-review" {
		t.Fatalf("last tag = %q", got)
	}
}

func TestJevClassifierFallsBackOnProviderFailure(t *testing.T) {
	fallback := fixedClassifier{result: Classification{Category: "billing", Provider: "local-ml"}}
	classifier := &JevClassifier{apiKey: "secret", model: "jev-latest", endpoint: "http://127.0.0.1:1", client: &http.Client{}, fallback: fallback}
	result := classifier.Classify(context.Background(), "invoice problem")
	if result.Provider != "local-ml" || result.Category != "billing" {
		t.Fatalf("fallback not used: %+v", result)
	}
}
