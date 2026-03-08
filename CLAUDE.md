# CLAUDE.md

## Project Overview

**PostmanLoadTester** is a Go-based load testing tool that runs Postman collections in parallel using [Newman](https://github.com/postmanlabs/newman) as the underlying test runner. It spawns multiple Newman processes as goroutines, parses their TeamCity-format output in real time, aggregates statistics, and displays a live-updating console table.

## Architecture

```
main.go                  Entry point, CLI flag parsing, worker orchestration
├── common/main.go       Shared data structures (TestStep, AggregatedTestStep, Worker interface, WorkerSettings)
├── scanner/main.go      Parses Newman's TeamCity reporter output via regex into TestStep events
├── aggregator/aggregator.go  Consumes TestStep events from channels, computes running statistics (thread-safe via sync.RWMutex)
├── console_printer/printer.go  Renders a live ASCII table of results (refreshes every 1s via uilive)
└── logger/logger.go     Initializes application.log and application.fail.log in the working directory
```

### Data Flow

1. `main.go` spawns N worker goroutines, each executing `newman run` as a subprocess
2. Each worker pipes stdout/stderr to `scanner.OutScanner()`, which parses TeamCity test events
3. Scanner sends `common.TestStep` messages into `aggregator.Source` (a buffered channel)
4. Aggregator maintains a map of `AggregatedTestStep` keyed by test name, computing avg duration, counts, and throughput
5. `ConsoleStatusPrinter` reads aggregator state every second via thread-safe getters and renders an ASCII table

### Key Interfaces

- `common.Worker` — defines `Close()` and `Run()` methods; implemented by `Aggregator` and `ConsoleStatusPrinter`
- `aggregator.GetStat()` / `aggregator.GetThroughput()` — thread-safe accessors for reading stats from the printer goroutine

## Prerequisites

- **Go 1.24+** (uses `go.mod` for dependency management)
- **Node.js 18+ / npm**
- **Newman 6.x:** `npm install -g newman`
- **TeamCity reporter:** `npm install -g newman-reporter-teamcity`

## Build & Run

```bash
# Build
go build

# Cross-compile for all platforms
bash make.sh
# Outputs to bin/postman-load-testing-{os}-{arch}

# Run
./postman-load-testing \
  -collection <collection_file_or_url> \
  -environment <environment_file_or_url> \
  -n <threads> \
  -i <iterations_per_thread> \
  -d <delay_ms> \
  -t <timeout_seconds>
```

### CLI Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `-collection` | Yes | — | URL or path to a Postman Collection |
| `-environment` | Yes | — | URL or path to a Postman Environment |
| `-n` | No | 1 | Number of parallel threads (must be > 0) |
| `-i` | No | 1 | Iterations per thread (must be > 0) |
| `-d` | No | 0 | Delay between requests in milliseconds (must be >= 0) |
| `-t` | No | 0 | Timeout per Newman run in seconds (0 = no timeout) |

## Testing

```bash
# Run all tests
go test -v -count=1 -timeout 120s ./...

# Run with race detector (recommended)
go test -race -v -count=1 -timeout 120s ./...
```

### Test structure

- `e2e_test.go` — end-to-end tests using real Newman + test HTTP server, plus unit tests for the scanner/aggregator pipeline
- `testdata/api_server.go` — embedded HTTP server (health, users CRUD, login) used by e2e tests
- `testdata/test_collection.json` — basic Postman v2.1.0 collection (5 requests, 10 assertions)
- `testdata/test_collection_modern.json` — modern Postman export format (structured URLs, folders, auth blocks)
- `testdata/test_environment.json` — environment template with `base_url` variable

### Test categories

| Test | Type | Description |
|------|------|-------------|
| `TestEndToEndWithNewman` | e2e | Single-worker run against test server |
| `TestEndToEndWithNewmanParallel` | e2e | 3 concurrent workers |
| `TestEndToEndModernFormat` | e2e | Modern Postman collection format |
| `TestPipelineScannerAggregator` | unit | Simulated TeamCity output parsing |
| `TestPipelineEmptyOutput` | unit | Empty output handling |
| `TestPipelineMalformedOutput` | unit | Malformed/garbage input resilience |
| `TestPipelineAllFailures` | unit | All-fail scenario |
| `TestCorrectRunningAverage` | unit | Verifies correct average calculation |

## Code Conventions

- **Package naming:** lowercase, underscore-separated for multi-word packages (`console_printer`, `out_scanner`)
- **Concurrency:** goroutines + channels for inter-component communication; `sync.WaitGroup` for worker lifecycle; `sync.RWMutex` for thread-safe stat access
- **Error handling:** errors are logged via `logger.Log`; workers return gracefully on failure instead of panicking
- **Logging:** uses Go's standard `log` package; two log files created at startup in the working directory

## Linting

```bash
golangci-lint run ./...
```

Configured linters (`.golangci.yml`): errcheck, govet, staticcheck, unused, ineffassign, gosec, misspell.

## Important Files

| File | Purpose |
|------|---------|
| `main.go` | Entry point — flag parsing, validation, worker pool, goroutine orchestration |
| `common/main.go` | Core types: `TestStep`, `AggregatedTestStep`, `Worker`, `WorkerSettings` |
| `aggregator/aggregator.go` | Thread-safe real-time statistics aggregation from channel-based event stream |
| `scanner/main.go` | Regex-based parser for Newman TeamCity output format |
| `console_printer/printer.go` | Live terminal table rendering with throughput stats |
| `logger/logger.go` | Log file initialization (`application.log`, `application.fail.log`) |
| `e2e_test.go` | All tests (e2e + unit) |
| `testdata/` | Test HTTP server, Postman collections and environments |
| `make.sh` | Cross-platform build script (darwin/linux/windows, 386/amd64) |
