package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"relaydesk/api/internal/domain"
)

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(ctx context.Context, databaseURL string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	store := &Postgres{pool: pool}
	if err = store.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (p *Postgres) Close() { p.pool.Close() }

func (p *Postgres) migrate(ctx context.Context) error {
	_, err := p.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tickets (
			id TEXT PRIMARY KEY,
			sequence INTEGER NOT NULL UNIQUE,
			title TEXT NOT NULL,
			summary TEXT NOT NULL,
			body TEXT NOT NULL,
			source TEXT NOT NULL,
			customer TEXT NOT NULL,
			customer_avatar TEXT NOT NULL,
			status TEXT NOT NULL,
			priority TEXT NOT NULL,
			category TEXT NOT NULL,
			assignee TEXT NOT NULL DEFAULT '',
			confidence DOUBLE PRECISION NOT NULL DEFAULT 0,
			ai_provider TEXT NOT NULL DEFAULT 'local-ml',
			tags JSONB NOT NULL DEFAULT '[]',
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			sla_deadline TIMESTAMPTZ NOT NULL,
			first_response TIMESTAMPTZ,
			external_issue TEXT NOT NULL DEFAULT '',
			events JSONB NOT NULL DEFAULT '[]'
		);
		CREATE INDEX IF NOT EXISTS tickets_updated_at_idx ON tickets (updated_at DESC);
		CREATE INDEX IF NOT EXISTS tickets_status_idx ON tickets (status);
		ALTER TABLE tickets ADD COLUMN IF NOT EXISTS ai_provider TEXT NOT NULL DEFAULT 'local-ml';
	`)
	return err
}

const ticketColumns = `id, sequence, title, summary, body, source, customer, customer_avatar,
	status, priority, category, assignee, confidence, ai_provider, tags, created_at, updated_at,
	sla_deadline, first_response, external_issue, events`

func scanTicket(row pgx.Row) (domain.Ticket, error) {
	var ticket domain.Ticket
	var tags, events []byte
	err := row.Scan(&ticket.ID, &ticket.Sequence, &ticket.Title, &ticket.Summary, &ticket.Body,
		&ticket.Source, &ticket.Customer, &ticket.CustomerAvatar, &ticket.Status, &ticket.Priority,
		&ticket.Category, &ticket.Assignee, &ticket.Confidence, &ticket.AIProvider, &tags, &ticket.CreatedAt,
		&ticket.UpdatedAt, &ticket.SLADeadline, &ticket.FirstResponse, &ticket.ExternalIssue, &events)
	if err != nil {
		return ticket, err
	}
	if err = json.Unmarshal(tags, &ticket.Tags); err != nil {
		return ticket, err
	}
	if err = json.Unmarshal(events, &ticket.Events); err != nil {
		return ticket, err
	}
	return ticket, nil
}

func (p *Postgres) List(ctx context.Context) ([]domain.Ticket, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+ticketColumns+` FROM tickets ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tickets := make([]domain.Ticket, 0)
	for rows.Next() {
		ticket, scanErr := scanTicket(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		tickets = append(tickets, ticket)
	}
	return tickets, rows.Err()
}

func (p *Postgres) Get(ctx context.Context, id string) (domain.Ticket, error) {
	ticket, err := scanTicket(p.pool.QueryRow(ctx, `SELECT `+ticketColumns+` FROM tickets WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ticket, ErrNotFound
	}
	return ticket, err
}

func (p *Postgres) Create(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
	tags, _ := json.Marshal(ticket.Tags)
	events, _ := json.Marshal(ticket.Events)
	_, err := p.pool.Exec(ctx, `INSERT INTO tickets (`+ticketColumns+`) VALUES
		($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
		ticket.ID, ticket.Sequence, ticket.Title, ticket.Summary, ticket.Body, ticket.Source,
		ticket.Customer, ticket.CustomerAvatar, ticket.Status, ticket.Priority, ticket.Category,
		ticket.Assignee, ticket.Confidence, ticket.AIProvider, tags, ticket.CreatedAt, ticket.UpdatedAt,
		ticket.SLADeadline, ticket.FirstResponse, ticket.ExternalIssue, events)
	return ticket, err
}

func (p *Postgres) Update(ctx context.Context, ticket domain.Ticket) (domain.Ticket, error) {
	tags, _ := json.Marshal(ticket.Tags)
	events, _ := json.Marshal(ticket.Events)
	row := p.pool.QueryRow(ctx, `UPDATE tickets SET title=$2, summary=$3, body=$4, source=$5,
		customer=$6, customer_avatar=$7, status=$8, priority=$9, category=$10, assignee=$11,
		confidence=$12, ai_provider=$13, tags=$14, updated_at=NOW(), sla_deadline=$15, first_response=$16,
		external_issue=$17, events=$18 WHERE id=$1 RETURNING `+ticketColumns,
		ticket.ID, ticket.Title, ticket.Summary, ticket.Body, ticket.Source, ticket.Customer,
		ticket.CustomerAvatar, ticket.Status, ticket.Priority, ticket.Category, ticket.Assignee,
		ticket.Confidence, ticket.AIProvider, tags, ticket.SLADeadline, ticket.FirstResponse, ticket.ExternalIssue, events)
	updated, err := scanTicket(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return updated, ErrNotFound
	}
	return updated, err
}
