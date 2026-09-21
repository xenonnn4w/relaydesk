FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /relaydesk-api ./cmd/server

FROM alpine:3.21
RUN adduser -D relaydesk
USER relaydesk
COPY --from=build /relaydesk-api /relaydesk-api
EXPOSE 8080
ENTRYPOINT ["/relaydesk-api"]
