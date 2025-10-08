# Repository Guidelines

## Project Structure & Module Organization
- Go modules in `go.mod`; main entry at `cmd/mcu-agent/main.go`.
- Internal packages live under `internal/` (e.g., `internal/pipeline`, `internal/media`, `internal/logging`, `internal/config`, `internal/metrics`).
- Tests colocated with code as `*_test.go` (e.g., `internal/pipeline/room_test.go`).
- Binaries output to `bin/` (ignored by clean). Scripts: `run`, `build`.

## Build, Test, and Development Commands
- `make build` — compile to `bin/mcu-agent`.
- `make run` — build then run the agent locally.
- `make test` — run all Go tests (`./...`).
- `make fmt` — format code via `gofmt -s -w .`.
- `make clean` — remove build artifacts.
- `make lint` — run `golangci-lint` (config in `.golangci.yaml/.yml`).

## Coding Style & Naming Conventions
- Language: Go 1.x with modules.
- Formatting: `gofmt -s -w .` (use `make fmt`).
- Linting: `golangci-lint` with repo configs.
- Indentation: tabs (Go default). Line length: prefer readability over hard wrap.
- Naming: packages `lowercase`, files `snake_case.go`, exported APIs use GoDoc style comments, tests `*_test.go` with `TestXxx`.

## Testing Guidelines
- Framework: standard `testing` package; use table tests where practical.
- Coverage: keep or increase coverage for touched packages; add tests alongside changes.
- Naming: test files `*_test.go`; tests `TestPackage_Feature` or `TestType_Method`.
- Run locally: `make test`; focused run: `go test ./internal/pipeline -run TestRoom`.

## Commit & Pull Request Guidelines
- Commits: concise, imperative subject (≤72 chars). Example: `pipeline: fix room selection policy edge-case`.
- Include rationale in body when non-trivial; reference issues like `Fixes #123`.
- PRs: clear description, scope of change, testing evidence (logs or `go test` output), and any config/env notes. Add screenshots only if relevant.

## Security & Configuration Tips
- Configuration via `.env`/`internal/config`; commit only `.env.example`.
- Prefer interfaces in `internal/*` to isolate external dependencies.
- Avoid logging secrets; use `internal/logging` helpers.

## Agent-Specific Notes
- Default binary: `bin/mcu-agent`; run with `./run` or `make run`.
- Container: `Dockerfile` + `docker-compose.yml` available for orchestration.
