package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"relaydesk/api/internal/domain"
	"relaydesk/api/internal/service"
	"relaydesk/api/internal/store"
)

type stubClassifier struct{}

func (stubClassifier) Classify(context.Context, string) service.Classification {
	return service.Classification{Title: "Webhook outage", Summary: "Production webhook outage", Category: "incident", Priority: domain.PriorityUrgent, Confidence: .95, Tags: []string{"incident"}}
}

func TestCreateThenClaimTicket(t *testing.T) {
	handler := New(store.NewMemory(nil), stubClassifier{})
	body := bytes.NewBufferString(`{"body":"Production webhooks are down","source":"slack","customer":"Maya"}`)
	create := httptest.NewRequest(http.MethodPost, "/api/intake", body)
	create.Header.Set("Content-Type", "application/json")
	createdResponse := httptest.NewRecorder()
	handler.ServeHTTP(createdResponse, create)
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201", createdResponse.Code)
	}
	var ticket domain.Ticket
	if err := json.NewDecoder(createdResponse.Body).Decode(&ticket); err != nil {
		t.Fatal(err)
	}
	if ticket.Priority != domain.PriorityUrgent || ticket.Category != "incident" {
		t.Fatalf("unexpected classification: %+v", ticket)
	}

	update := httptest.NewRequest(http.MethodPatch, "/api/tickets/"+ticket.ID, bytes.NewBufferString(`{"status":"in_progress","assignee":"You"}`))
	update.Header.Set("Content-Type", "application/json")
	updatedResponse := httptest.NewRecorder()
	handler.ServeHTTP(updatedResponse, update)
	if updatedResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200: %s", updatedResponse.Code, updatedResponse.Body.String())
	}
	if err := json.NewDecoder(updatedResponse.Body).Decode(&ticket); err != nil {
		t.Fatal(err)
	}
	if ticket.Status != domain.StatusInProgress || ticket.Assignee != "You" {
		t.Fatalf("ticket was not claimed: %+v", ticket)
	}
	if len(ticket.Events) != 3 {
		t.Fatalf("event count = %d, want 3", len(ticket.Events))
	}
	_ = time.Now()
}

func TestRejectsInvalidStatusTransition(t *testing.T) {
	now := time.Now()
	ticket := domain.Ticket{ID: "tkt_1", Status: domain.StatusNew, CreatedAt: now, UpdatedAt: now, SLADeadline: now.Add(time.Hour)}
	handler := New(store.NewMemory([]domain.Ticket{ticket}), stubClassifier{})
	request := httptest.NewRequest(http.MethodPatch, "/api/tickets/tkt_1", bytes.NewBufferString(`{"status":"waiting_customer"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", response.Code)
	}
}

func TestCustomerMessageIsPersistedAndReadable(t *testing.T) {
	now := time.Now()
	ticket := domain.Ticket{ID: "tkt_chat", Customer: "Asha", Status: domain.StatusWaitingCustomer, CreatedAt: now, UpdatedAt: now, SLADeadline: now.Add(time.Hour), Events: []domain.Event{}}
	handler := New(store.NewMemory([]domain.Ticket{ticket}), stubClassifier{})

	message := httptest.NewRequest(http.MethodPost, "/api/tickets/tkt_chat/customer-message", bytes.NewBufferString(`{"body":"I am still seeing the error"}`))
	messageResponse := httptest.NewRecorder()
	handler.ServeHTTP(messageResponse, message)
	if messageResponse.Code != http.StatusOK {
		t.Fatalf("message status = %d, want 200", messageResponse.Code)
	}

	read := httptest.NewRequest(http.MethodGet, "/api/tickets/tkt_chat", nil)
	readResponse := httptest.NewRecorder()
	handler.ServeHTTP(readResponse, read)
	if readResponse.Code != http.StatusOK {
		t.Fatalf("read status = %d, want 200", readResponse.Code)
	}
	if err := json.NewDecoder(readResponse.Body).Decode(&ticket); err != nil {
		t.Fatal(err)
	}
	if ticket.Status != domain.StatusInProgress {
		t.Fatalf("status = %q, want in_progress", ticket.Status)
	}
	if len(ticket.Events) != 1 || ticket.Events[0].Type != "customer_message" {
		t.Fatalf("unexpected events: %+v", ticket.Events)
	}
}
