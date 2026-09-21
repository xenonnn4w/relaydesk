package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"relaydesk/api/internal/domain"
	"relaydesk/api/internal/service"
	"relaydesk/api/internal/store"
)

type API struct {
	store      store.TicketStore
	classifier service.Classifier
	now        func() time.Time
}

type updateTicketRequest struct {
	Status   *domain.Status   `json:"status"`
	Priority *domain.Priority `json:"priority"`
	Assignee *string          `json:"assignee"`
}

type intakeRequest struct {
	Body     string `json:"body"`
	Source   string `json:"source"`
	Customer string `json:"customer"`
}

func New(ticketStore store.TicketStore, classifier service.Classifier) http.Handler {
	api := &API{store: ticketStore, classifier: classifier, now: func() time.Time { return time.Now().UTC() }}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", api.health)
	mux.HandleFunc("GET /api/tickets", api.listTickets)
	mux.HandleFunc("GET /api/tickets/{id}", api.getTicket)
	mux.HandleFunc("POST /api/intake", api.createTicket)
	mux.HandleFunc("PATCH /api/tickets/{id}", api.updateTicket)
	mux.HandleFunc("POST /api/tickets/{id}/escalate", api.escalate)
	mux.HandleFunc("POST /api/tickets/{id}/reply", api.reply)
	mux.HandleFunc("POST /api/tickets/{id}/customer-message", api.customerMessage)
	return cors(requestLog(mux))
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) listTickets(w http.ResponseWriter, r *http.Request) {
	tickets, err := a.store.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	now := a.now()
	type ticketResponse struct {
		domain.Ticket
		SLARisk string `json:"slaRisk"`
	}
	response := make([]ticketResponse, 0, len(tickets))
	for _, ticket := range tickets {
		response = append(response, ticketResponse{Ticket: ticket, SLARisk: ticket.SLARisk(now)})
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *API) getTicket(w http.ResponseWriter, r *http.Request) {
	ticket, err := a.store.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, struct {
		domain.Ticket
		SLARisk string `json:"slaRisk"`
	}{ticket, ticket.SLARisk(a.now())})
}

func (a *API) createTicket(w http.ResponseWriter, r *http.Request) {
	var input intakeRequest
	if err := decodeJSON(r, &input); err != nil || strings.TrimSpace(input.Body) == "" {
		writeError(w, http.StatusBadRequest, "body is required")
		return
	}
	if input.Source == "" {
		input.Source = "web"
	}
	if input.Customer == "" {
		input.Customer = "Anonymous"
	}

	now := a.now()
	classification := a.classifier.Classify(r.Context(), input.Body)
	existing, _ := a.store.List(r.Context())
	ticket := domain.Ticket{
		ID: id("tkt"), Sequence: 1041 + len(existing), Title: classification.Title,
		Summary: classification.Summary, Body: input.Body, Source: input.Source,
		Customer: input.Customer, CustomerAvatar: initials(input.Customer), Status: domain.StatusNew,
		Priority: classification.Priority, Category: classification.Category, Confidence: classification.Confidence, AIProvider: classification.Provider,
		Tags: classification.Tags, CreatedAt: now, UpdatedAt: now, SLADeadline: now.Add(slaFor(classification.Priority)),
		Events: []domain.Event{{ID: id("evt"), Type: "created", Body: "Request captured from " + input.Source, Actor: "RelayDesk", CreatedAt: now}},
	}
	created, err := a.store.Create(r.Context(), ticket)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) customerMessage(w http.ResponseWriter, r *http.Request) {
	ticket, err := a.store.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}
	var input struct {
		Body string `json:"body"`
	}
	if decodeJSON(r, &input) != nil || strings.TrimSpace(input.Body) == "" {
		writeError(w, http.StatusBadRequest, "message body is required")
		return
	}
	now := a.now()
	if ticket.Status == domain.StatusWaitingCustomer {
		ticket.Status = domain.StatusInProgress
	}
	ticket.Events = append(ticket.Events, domain.Event{ID: id("evt"), Type: "customer_message", Body: strings.TrimSpace(input.Body), Actor: ticket.Customer, CreatedAt: now})
	updated, err := a.store.Update(r.Context(), ticket)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) updateTicket(w http.ResponseWriter, r *http.Request) {
	ticket, err := a.store.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var input updateTicketRequest
	if decodeJSON(r, &input) != nil {
		writeError(w, http.StatusBadRequest, "invalid update")
		return
	}
	changes := make([]string, 0, 3)
	if input.Status != nil {
		if !domain.ValidTransition(ticket.Status, *input.Status) {
			writeError(w, http.StatusConflict, "invalid status transition")
			return
		}
		if ticket.Status != *input.Status {
			changes = append(changes, "Status changed to "+string(*input.Status))
			ticket.Status = *input.Status
		}
	}
	if input.Priority != nil && ticket.Priority != *input.Priority {
		changes = append(changes, "Priority changed to "+string(*input.Priority))
		ticket.Priority = *input.Priority
	}
	if input.Assignee != nil && ticket.Assignee != *input.Assignee {
		changes = append(changes, "Assigned to "+*input.Assignee)
		ticket.Assignee = *input.Assignee
	}
	for _, change := range changes {
		ticket.Events = append(ticket.Events, domain.Event{ID: id("evt"), Type: "updated", Body: change, Actor: "You", CreatedAt: a.now()})
	}
	updated, err := a.store.Update(r.Context(), ticket)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) escalate(w http.ResponseWriter, r *http.Request) {
	ticket, err := a.store.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}
	if ticket.ExternalIssue == "" {
		ticket.ExternalIssue = "GH-" + strconv.Itoa(380+ticket.Sequence%100)
		ticket.Events = append(ticket.Events, domain.Event{ID: id("evt"), Type: "escalated", Body: "Engineering issue " + ticket.ExternalIssue + " created", Actor: "RelayDesk", CreatedAt: a.now()})
	}
	updated, _ := a.store.Update(r.Context(), ticket)
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) reply(w http.ResponseWriter, r *http.Request) {
	ticket, err := a.store.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}
	var input struct {
		Body string `json:"body"`
	}
	if decodeJSON(r, &input) != nil || strings.TrimSpace(input.Body) == "" {
		writeError(w, http.StatusBadRequest, "reply body is required")
		return
	}
	now := a.now()
	if ticket.FirstResponse == nil {
		ticket.FirstResponse = &now
	}
	if ticket.Status == domain.StatusNew {
		ticket.Status = domain.StatusInProgress
	}
	ticket.Events = append(ticket.Events, domain.Event{ID: id("evt"), Type: "reply", Body: input.Body, Actor: "You", CreatedAt: now})
	updated, _ := a.store.Update(r.Context(), ticket)
	writeJSON(w, http.StatusOK, updated)
}

func slaFor(priority domain.Priority) time.Duration {
	switch priority {
	case domain.PriorityUrgent:
		return 30 * time.Minute
	case domain.PriorityHigh:
		return 2 * time.Hour
	case domain.PriorityLow:
		return 24 * time.Hour
	default:
		return 8 * time.Hour
	}
}

func initials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "?"
	}
	if len(parts) == 1 {
		return strings.ToUpper(string([]rune(parts[0])[0]))
	}
	return strings.ToUpper(string([]rune(parts[0])[0]) + string([]rune(parts[len(parts)-1])[0]))
}

func id(prefix string) string {
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

func decodeJSON(r *http.Request, value any) error {
	defer r.Body.Close()
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(value)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestLog(next http.Handler) http.Handler { return next }

var _ = context.Background
