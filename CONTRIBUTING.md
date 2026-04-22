# Contributing

Contributions are welcome: bug reports, documentation fixes, tests, and focused feature PRs.

## Before you start

- **Scope:** Keep changes aligned with a single concern per pull request.
- **Tests:** Run `go test ./...` from the repo root. Integration tests use `-tags=integration` and Docker (see `integration/`).
- **API behavior:** If you change HTTP routes or JSON shapes, update OpenAPI under `openapi/` and any affected docs in `docs/`.

## Development setup

- Copy `.env.example` to `.env` and adjust; see the main [README.md](README.md) and [docs/DEPLOY.md](docs/DEPLOY.md) for Compose and local runs.
- Go version: see `go.mod` (toolchain line).

## Code style

- Match existing formatting (`gofmt`) and patterns in nearby files.
- Prefer clear error handling; **do not return raw internal errors in HTTP 5xx bodies** — use `WriteProblem` in `internal/api/problem.go`, which redacts server-side details for clients while logging them server-side.

## Security

See [SECURITY.md](SECURITY.md) for vulnerability reporting. Do not post exploit details in public issues.
