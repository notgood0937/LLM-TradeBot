FROM golang:1.22-bookworm AS builder

WORKDIR /app
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
COPY web ./web
COPY start_go.sh ./start_go.sh
RUN go build -o /out/tradebotd ./cmd/tradebotd

FROM debian:bookworm-slim

WORKDIR /app
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /out/tradebotd /app/tradebotd
COPY web /app/web
COPY config.example.yaml /app/config.example.yaml
COPY start_go.sh /app/start_go.sh

EXPOSE 8000

CMD ["./tradebotd"]
