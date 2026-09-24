# RelayDesk

RelayDesk is an AI-assisted support operations console built to demonstrate the engineering problems behind conversational support products: multi-source ingestion, structured ticket state, SLA visibility, ML-assisted triage, immutable activity history, and engineering escalation.

It is intentionally a focused product slice rather than a ClearFeed clone.

## What works

- Unified queue for Slack, Discord, and web-chat requests
- Working two-way customer web chat at `/chat`
- Search plus assigned, unassigned, SLA-attention, and resolved views
- Guarded ticket state transitions, priority, and assignment
- First-response and SLA tracking
- Optional Jev typed-decision classification with automatic local ML fallback
- ML-assisted category, priority, title, and summary generation
- Safety rail that promotes likely production incidents
- Public replies with an immutable activity timeline
- GitHub-style engineering escalation
- PostgreSQL persistence in Docker and in-memory fallback for local demos
- Responsive operations-console interface

## Architecture

```text
Slack / Discord / Web simulator
              |
              v
       Go HTTP API (chi-free)
     validation + state machine
              |
      +-------+---------+
      |                 |
      v                 v
 PostgreSQL       Python FastAPI
 tickets/events   TF-IDF + logistic
      |           regression model
      +-------+---------+
              |
              v
     React + TypeScript UI
```

The Go service uses only the standard library for HTTP routing. PostgreSQL access is isolated behind a store interface, so the same application can run with an in-memory store when `DATABASE_URL` is absent.

## Run the complete stack

Requirements: Docker and Docker Compose.

```bash
docker compose up --build
```

Open `http://localhost:5173`.

- Agent console: `http://localhost:5173/`
- Customer web chat: `http://localhost:5173/chat`

### Enable Jev

Jev uses a TypeSafe API key, not an OpenAI key. Keep the key in your shell or `.env`; never commit it.

```bash
export JEV_API_KEY='your-typesafe-key'
docker compose up -d --build api
```

When no Jev key is configured—or when Jev times out or returns an invalid decision—the API automatically uses the local Python classifier. Every ticket records the provider used as `jev`, `local-ml`, or `rule-fallback`.

## Run services locally

The API works without infrastructure by falling back to seeded in-memory data. If the Python service is unavailable, it also applies a deterministic triage fallback.

```bash
# terminal 1
cd api && go run ./cmd/server

# terminal 2
cd web && npm install && npm run dev

# optional terminal 3: full ML classifier
cd ai
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
uvicorn app.main:app --reload
```

## API surface

| Method | Route | Purpose |
|---|---|---|
| `GET` | `/health` | Liveness check |
| `GET` | `/api/tickets` | Queue with computed SLA risk |
| `GET` | `/api/tickets/{id}` | Poll one ticket and its conversation |
| `POST` | `/api/intake` | Normalize and classify a new request |
| `PATCH` | `/api/tickets/{id}` | Change state, priority, or assignee |
| `POST` | `/api/tickets/{id}/reply` | Record a support response |
| `POST` | `/api/tickets/{id}/customer-message` | Append a customer web-chat message |
| `POST` | `/api/tickets/{id}/escalate` | Create/link an engineering issue |

## Verification

```bash
cd api && go test ./...
cd web && npm test && npm run build
cd ai && python -m compileall app tests
docker compose config
```

## Deliberate MVP boundaries

- The engineering issue integration is simulated; connecting the GitHub App and verifying its webhook is the next vertical slice.
- The web-chat channel is functional. Production Slack/Discord OAuth and webhook verification are not included yet.
- The bundled classifier is trained on a tiny transparent demonstration dataset. Production work would add labelled evaluation data, drift monitoring, and confidence calibration.
- Redis-based delayed SLA jobs and a pgvector knowledge base are planned follow-ups, not unused dependencies in this release.
