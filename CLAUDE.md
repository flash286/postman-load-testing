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
└── logger/logger.go     Initializes application.log and application.fail.log in the working directory
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

- **Go** (project uses `go get` / `go build` — no `go.mod` present)
- **Node.js / npm**
- **Newman:** `npm install -g newman`
- **TeamCity reporter:** `npm install -g newman-reporter-teamcity`

## Build & Run

```bash
# Install dependencies
go get

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

## Testing

There are currently no tests (`*_test.go` files) in this project.

## Code Conventions

- **Package naming:** lowercase, underscore-separated for multi-word packages (`console_printer`, `out_scanner`)
- **Concurrency:** goroutines + channels for inter-component communication; `sync.WaitGroup` for worker lifecycle
- **Error handling:** panics on critical failures (e.g., subprocess start failure); no graceful error propagation
- **Logging:** uses Go's standard `log` package; two log files created at startup in the working directory
- **No linting/formatting tools** are configured; follow standard `gofmt` conventions when making changes

## Important Files

| File | Purpose |
|------|---------|
| `main.go` | Entry point — flag parsing, worker pool, goroutine orchestration |
| `common/main.go` | Core types: `TestStep`, `AggregatedTestStep`, `Worker`, `WorkerSettings` |
| `aggregator/aggregator.go` | Real-time statistics aggregation from channel-based event stream |
| `scanner/main.go` | Regex-based parser for Newman TeamCity output format |
| `console_printer/printer.go` | Live terminal table rendering with throughput stats |
| `logger/logger.go` | Log file initialization (`application.log`, `application.fail.log`) |
| `make.sh` | Cross-platform build script (darwin/linux/windows, 386/amd64) |
