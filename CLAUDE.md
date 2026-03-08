# CLAUDE.md

## Project Overview

**PostmanLoadTester** is a Go-based load testing tool that runs Postman collections in parallel using [Newman](https://github.com/postmanlabs/newman) as the underlying test runner. It spawns multiple Newman processes as goroutines, parses their TeamCity-format output in real time, aggregates statistics, and displays a live-updating console table.

**Status:** Experimental — use at your own risk.

## Architecture

```
main.go                  Entry point, CLI flag parsing, worker orchestration
├── common/main.go       Shared data structures (TestStep, AggregatedTestStep, Worker interface, WorkerSettings)
├── scanner/main.go      Parses Newman's TeamCity reporter output via regex into TestStep events
├── aggregator/aggregator.go  Consumes TestStep events from channels, computes running statistics
├── console_printer/printer.go  Renders a live ASCII table of results (refreshes every 1s via uilive)
├── logger/logger.go     Initializes application.log and application.fail.log in the working directory
└── testdata/            Test HTTP server and Postman collection for e2e tests
    ├── api_server.go    Simple REST API (health, users, login endpoints)
    ├── test_collection.json   Postman collection with 5 requests and assertions
    └── test_environment.json  Environment template with base_url variable
```

### Data Flow

1. `main.go` spawns N worker goroutines, each executing `newman run` as a subprocess
2. Each worker pipes stdout/stderr to `scanner.OutScanner()`, which parses TeamCity test events
3. Scanner sends `common.TestStep` messages into `aggregator.Source` (a buffered channel)
4. Aggregator maintains a map of `AggregatedTestStep` keyed by test name, computing avg duration, counts, and throughput
5. `ConsoleStatusPrinter` reads aggregator state every second and renders an ASCII table via `uilive`

### Key Interfaces

- `common.Worker` — defines `Close()` and `Run()` methods; implemented by `Aggregator` and `ConsoleStatusPrinter`

## Prerequisites

- **Go** 1.24+
- **Node.js** 18+ / npm
- **Newman:** `npm install -g newman`
- **TeamCity reporter:** `npm install -g newman-reporter-teamcity`

## Build & Run

```bash
# Build
go build -o postman-load-testing .

# Cross-compile for all platforms
bash make.sh
# Outputs to bin/postman-load-testing-{os}-{arch}

# Run
./postman-load-testing \
  -collection <collection_file_or_url> \
  -environment <environment_file_or_url> \
  -n <threads> \
  -i <iterations_per_thread> \
  -d <delay_ms>
```

### CLI Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `-collection` | Yes | — | URL or path to a Postman Collection |
| `-environment` | Yes | — | URL or path to a Postman Environment |
| `-n` | No | 1 | Number of parallel threads (goroutines) |
| `-i` | No | 1 | Iterations per thread |
| `-d` | No | 0 | Delay between requests in milliseconds |

## Linting

```bash
golangci-lint run ./...
```

Configuration: `.golangci.yml` — enables errcheck, govet, staticcheck, unused, ineffassign.

## Testing

```bash
go test -v -count=1 -timeout 120s ./...
```

The test suite includes:
- **`TestEndToEndWithNewman`** — starts a real HTTP server, runs Newman, validates aggregated results
- **`TestEndToEndWithNewmanParallel`** — 3 concurrent workers hitting the test server
- **`TestPipelineScannerAggregator`** — scanner → aggregator with simulated TeamCity output
- **`TestPipelineEmptyOutput`** — edge case: empty Newman output

Tests in `e2e_test.go` require Newman to be installed; they skip gracefully if missing.

## CI/CD

GitHub Actions (`.github/workflows/ci.yml`) runs on push to `master` and on PRs:
1. **Lint** — golangci-lint
2. **Test** — full suite with Newman
3. **Build** — cross-compilation matrix (linux/darwin/windows × amd64/386)

## Code Conventions

- **Package naming:** lowercase, underscore-separated for multi-word packages (`console_printer`, `out_scanner`)
- **Concurrency:** goroutines + channels for inter-component communication; `sync.WaitGroup` for worker lifecycle
- **Error handling:** panics on critical failures (e.g., subprocess start failure); handle errors explicitly everywhere else
- **Logging:** uses Go's standard `log` package; two log files created at startup in the working directory
- **Linting:** all code must pass `golangci-lint run ./...` before committing
- **Formatting:** follow standard `gofmt` conventions

## Important Files

| File | Purpose |
|------|---------|
| `main.go` | Entry point — flag parsing, worker pool, goroutine orchestration |
| `common/main.go` | Core types: `TestStep`, `AggregatedTestStep`, `Worker`, `WorkerSettings` |
| `aggregator/aggregator.go` | Real-time statistics aggregation from channel-based event stream |
| `scanner/main.go` | Regex-based parser for Newman TeamCity output format |
| `console_printer/printer.go` | Live terminal table rendering with throughput stats |
| `logger/logger.go` | Log file initialization (`application.log`, `application.fail.log`) |
| `e2e_test.go` | End-to-end and pipeline tests |
| `testdata/api_server.go` | Test HTTP API server for e2e tests |
| `testdata/test_collection.json` | Postman collection used by e2e tests |
| `make.sh` | Cross-platform build script (darwin/linux/windows, 386/amd64) |
| `.golangci.yml` | Linter configuration |
| `.github/workflows/ci.yml` | GitHub Actions CI pipeline |
