package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"relaydesk/api/internal/domain"
	"relaydesk/api/internal/httpapi"
	"relaydesk/api/internal/service"
	"relaydesk/api/internal/store"
)

func main() {
	now := time.Now().UTC()
	seeds := seedTickets(now)
	var ticketStore store.TicketStore = store.NewMemory(seeds)
	var closeStore func()
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		postgres, err := store.NewPostgres(context.Background(), databaseURL)
		if err != nil {
			log.Printf("database unavailable; using in-memory store: %v", err)
		} else {
			closeStore = postgres.Close
			existing, listErr := postgres.List(context.Background())
			if listErr != nil {
				log.Fatal(listErr)
			}
			if len(existing) == 0 {
				for _, ticket := range seeds {
					if _, createErr := postgres.Create(context.Background(), ticket); createErr != nil {
						log.Fatal(createErr)
					}
				}
			}
			ticketStore = postgres
			log.Printf("connected to PostgreSQL")
		}
	}
	if closeStore != nil {
		defer closeStore()
	}
	classifierURL := env("AI_SERVICE_URL", "http://localhost:8000")
	localClassifier := service.NewHTTPClassifier(classifierURL)
	classifier := service.NewJevClassifier(os.Getenv("JEV_API_KEY"), env("JEV_MODEL", "jev-latest"), localClassifier)
	handler := httpapi.New(ticketStore, classifier)
	port := env("API_PORT", "8080")
	server := &http.Server{Addr: ":" + port, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("RelayDesk API listening on :%s", port)
	log.Fatal(server.ListenAndServe())
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func seedTickets(now time.Time) []domain.Ticket {
	minutes := func(n int) time.Time { return now.Add(time.Duration(n) * time.Minute) }
	return []domain.Ticket{
		{
			ID: "tkt_orbital", Sequence: 1038, Title: "Production webhook deliveries are failing", Summary: "Webhook deliveries began returning 502s after the latest deployment.",
			Body:   "Hey team — our production webhook deliveries have been failing with 502 responses since the deploy about twenty minutes ago. This is blocking order updates for every customer. Can someone take a look urgently?",
			Source: "slack", Customer: "Maya Chen", CustomerAvatar: "MC", Status: domain.StatusNew, Priority: domain.PriorityUrgent, Category: "incident", Confidence: .96,
			AIProvider: "seeded-demo",
			Tags:       []string{"webhooks", "production"}, CreatedAt: minutes(-22), UpdatedAt: minutes(-4), SLADeadline: minutes(8),
			Events: []domain.Event{{ID: "evt_1", Type: "created", Body: "Request captured from #orbital-support", Actor: "RelayDesk", CreatedAt: minutes(-22)}, {ID: "evt_2", Type: "classified", Body: "Marked urgent · Incident · 96% confidence", Actor: "AI triage", CreatedAt: minutes(-21)}},
		},
		{
			ID: "tkt_northstar", Sequence: 1037, Title: "SSO users cannot access the analytics workspace", Summary: "Newly provisioned SSO users receive a permission error opening analytics.",
			Body:   "Three people we added through Okta this morning can sign in, but they get a permission denied screen whenever they open Analytics. Existing members are fine.",
			Source: "web", Customer: "Arjun Mehta", CustomerAvatar: "AM", Status: domain.StatusInProgress, Priority: domain.PriorityHigh, Category: "access", Assignee: "You", Confidence: .89,
			AIProvider: "seeded-demo",
			Tags:       []string{"sso", "permissions"}, CreatedAt: minutes(-78), UpdatedAt: minutes(-12), SLADeadline: minutes(42), ExternalIssue: "GH-417",
			Events: []domain.Event{{ID: "evt_3", Type: "created", Body: "Request submitted from the support portal", Actor: "RelayDesk", CreatedAt: minutes(-78)}, {ID: "evt_4", Type: "updated", Body: "Assigned to You", Actor: "You", CreatedAt: minutes(-51)}, {ID: "evt_5", Type: "escalated", Body: "Engineering issue GH-417 created", Actor: "RelayDesk", CreatedAt: minutes(-12)}},
		},
		{
			ID: "tkt_luma", Sequence: 1036, Title: "Invoice includes duplicate workspace charge", Summary: "The September invoice appears to contain the same workspace charge twice.",
			Body:   "Could you check invoice INV-8841? The Acme APAC workspace appears twice and the total is higher than expected. I've attached the invoice in the original thread.",
			Source: "discord", Customer: "Nora Ellis", CustomerAvatar: "NE", Status: domain.StatusWaitingCustomer, Priority: domain.PriorityHigh, Category: "billing", Assignee: "Priya", Confidence: .92,
			AIProvider: "seeded-demo",
			Tags:       []string{"invoice", "billing"}, CreatedAt: minutes(-164), UpdatedAt: minutes(-35), SLADeadline: minutes(-4),
			Events: []domain.Event{{ID: "evt_6", Type: "created", Body: "Request captured from Discord", Actor: "RelayDesk", CreatedAt: minutes(-164)}, {ID: "evt_7", Type: "reply", Body: "We are checking the invoice line items with our billing team.", Actor: "Priya", CreatedAt: minutes(-35)}},
		},
		{
			ID: "tkt_finch", Sequence: 1035, Title: "Export filtered tickets as CSV", Summary: "Customer would like to export the currently filtered ticket view.",
			Body:   "It would be useful if the export action respected our active filters instead of exporting the whole workspace. Is this something on the roadmap?",
			Source: "slack", Customer: "Liam Brooks", CustomerAvatar: "LB", Status: domain.StatusResolved, Priority: domain.PriorityLow, Category: "feature", Assignee: "You", Confidence: .83,
			AIProvider: "seeded-demo",
			Tags:       []string{"export", "feature-request"}, CreatedAt: minutes(-420), UpdatedAt: minutes(-90), SLADeadline: minutes(-180), ExternalIssue: "GH-409",
			Events: []domain.Event{{ID: "evt_8", Type: "created", Body: "Request captured from #finch-support", Actor: "RelayDesk", CreatedAt: minutes(-420)}, {ID: "evt_9", Type: "reply", Body: "Added to our product backlog for review.", Actor: "You", CreatedAt: minutes(-120)}, {ID: "evt_10", Type: "updated", Body: "Status changed to resolved", Actor: "You", CreatedAt: minutes(-90)}},
		},
	}
}
