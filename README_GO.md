# Go Runtime Notes

This repository now runs on Go only.

## Entrypoints

- `./start.sh`
- `./start_go.sh`
- `go run ./cmd/tradebotd`
- `go run ./cmd/tradebot`

## Server

- Host: `127.0.0.1`
- Port: `8000`

## Build

```bash
go build ./cmd/tradebot ./cmd/tradebotd
```

## Main Packages

- `cmd/tradebotd`
- `cmd/tradebot`
- `internal/server`
- `internal/binance`
- `internal/llm`
- `internal/execution`
- `internal/risk`
- `internal/selector`
- `internal/state`

## Notes

- Python source code has been removed from the repository.
- The active deployment/runtime path is Go.
