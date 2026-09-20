.PHONY: dev-api dev-web dev-ai test

dev-api:
	cd api && go run ./cmd/server

dev-web:
	cd web && npm run dev

dev-ai:
	cd ai && uvicorn app.main:app --reload

test:
	cd api && go test ./...
	cd web && npm run test

