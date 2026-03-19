FROM golang:1.22-bookworm AS builder

WORKDIR /app
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
COPY web ./web
RUN go build -o /out/tradebotd ./cmd/tradebotd

FROM debian:bookworm-slim

WORKDIR /app
RUN apt-get update && apt-get install -y ca-certificates curl && rm -rf /var/lib/apt/lists/*
COPY --from=builder /out/tradebotd /app/tradebotd
COPY web /app/web

EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=10s --start-period=20s --retries=3 \
    CMD curl -fsS http://localhost:8000/api/health || exit 1

CMD ["./tradebotd"]
