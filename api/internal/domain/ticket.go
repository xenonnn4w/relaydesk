package domain

import "time"

type Status string

const (
	StatusNew             Status = "new"
	StatusInProgress      Status = "in_progress"
	StatusWaitingCustomer Status = "waiting_customer"
	StatusResolved        Status = "resolved"
)

type Priority string

const (
	PriorityUrgent Priority = "urgent"
	PriorityHigh   Priority = "high"
	PriorityNormal Priority = "normal"
	PriorityLow    Priority = "low"
)

type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Body      string    `json:"body"`
	Actor     string    `json:"actor"`
	CreatedAt time.Time `json:"createdAt"`
}

type Ticket struct {
	ID             string     `json:"id"`
	Sequence       int        `json:"sequence"`
	Title          string     `json:"title"`
	Summary        string     `json:"summary"`
	Body           string     `json:"body"`
	Source         string     `json:"source"`
	Customer       string     `json:"customer"`
	CustomerAvatar string     `json:"customerAvatar"`
	Status         Status     `json:"status"`
	Priority       Priority   `json:"priority"`
	Category       string     `json:"category"`
	Assignee       string     `json:"assignee"`
	Confidence     float64    `json:"confidence"`
	AIProvider     string     `json:"aiProvider"`
	Tags           []string   `json:"tags"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	SLADeadline    time.Time  `json:"slaDeadline"`
	FirstResponse  *time.Time `json:"firstResponse,omitempty"`
	ExternalIssue  string     `json:"externalIssue,omitempty"`
	Events         []Event    `json:"events"`
}

func (t Ticket) SLARisk(now time.Time) string {
	if t.Status == StatusResolved {
		return "met"
	}
	remaining := t.SLADeadline.Sub(now)
	if remaining <= 0 {
		return "breached"
	}
	if remaining <= 30*time.Minute {
		return "at_risk"
	}
	return "healthy"
}

func ValidTransition(from, to Status) bool {
	if from == to {
		return true
	}
	allowed := map[Status]map[Status]bool{
		StatusNew:             {StatusInProgress: true, StatusResolved: true},
		StatusInProgress:      {StatusWaitingCustomer: true, StatusResolved: true},
		StatusWaitingCustomer: {StatusInProgress: true, StatusResolved: true},
		StatusResolved:        {StatusInProgress: true},
	}
	return allowed[from][to]
}
