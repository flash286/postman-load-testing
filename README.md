# PostmanLoadTester

[![CI](https://github.com/flash286/postman-load-testing/actions/workflows/ci.yml/badge.svg)](https://github.com/flash286/postman-load-testing/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Newman](https://img.shields.io/badge/Newman-6.x-FF6C37?logo=postman&logoColor=white)](https://www.npmjs.com/package/newman)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

A Go-based load testing tool that runs [Postman](https://www.postman.com/) collections in parallel using [Newman](https://github.com/postmanlabs/newman). Spawn multiple concurrent workers, aggregate results in real time, and view live statistics in your terminal.

## Console Output

**Startup banner** — shows your test configuration at a glance:

```
╭──────────────────────────────────────────────────────────────────╮
│  POSTMAN LOAD TESTER                                             │
│                                                                  │
│  Collection:  my-api-tests.postman_collection.json               │
│  Environment: staging.postman_environment.json                   │
│  Workers: 10   Iterations: 5   Delay: 0ms                       │
│                                                                  │
│  Log: application.log  |  Fail Log: application.fail.log         │
╰──────────────────────────────────────────────────────────────────╯
```

**Live dashboard** — updates every second with animated spinner, color-coded metrics, and visual progress bars:

```
╭──────────────────────────────────────────────────────────────────╮
│  ◐ LOAD TEST RUNNING                                             │
│    Workers 10  |  Elapsed 00:12  |  RPS 42                       │
│                                                                  │
│   NAME              AVG        OK      FAIL     TOTAL  RATE      │
│   ──────────────────────────────────────────────────────────────  │
│   Health Check      12ms      150         0       150  ██████████ │
│   Get Users         45ms      148         2       150  █████████░ │
│   Get User By ID    23ms      150         0       150  ██████████ │
│   Login             34ms      150         0       150  ██████████ │
│   Create User       67ms      149         1       150  █████████░ │
│   ──────────────────────────────────────────────────────────────  │
│   Total: 750    Success Rate: 99.6%    Throughput: 42 rps        │
╰──────────────────────────────────────────────────────────────────╯
```

**Final results** — displayed with a double-border frame when the test completes:

```
╔══════════════════════════════════════════════════════════════════╗
║  LOAD TEST COMPLETE                                              ║
║    Workers 10  |  Elapsed 01:47  |  RPS 42                       ║
║                                                                  ║
║   NAME              AVG        OK      FAIL     TOTAL  RATE      ║
║   ──────────────────────────────────────────────────────────────  ║
║   Health Check      12ms      500         0       500  ██████████ ║
║   Get Users         45ms      497         3       500  █████████░ ║
║   Get User By ID    23ms      500         0       500  ██████████ ║
║   Login             34ms      500         0       500  ██████████ ║
║   Create User       67ms      498         2       500  █████████░ ║
║   ──────────────────────────────────────────────────────────────  ║
║   Total: 2500    Success Rate: 99.8%    Throughput: 42 rps       ║
╚══════════════════════════════════════════════════════════════════╝
```

> Colors in terminal: names in **white**, durations in **yellow**, success counts in **green**, failures in **red**, progress bars **green**/​**red**, metrics in **cyan**. Success rate turns yellow below 100% and red below 95%.

## Features

- **Parallel execution** — run N Newman instances concurrently via goroutines
- **Real-time dashboard** — live-updating ASCII table with per-endpoint stats
- **Aggregated metrics** — average duration, success/fail counts, requests per second
- **Flexible input** — accepts local files or URLs for collections and environments
- **Cross-platform** — builds for Linux, macOS, and Windows (amd64/386)

## Prerequisites

- [Go](https://go.dev/dl/) 1.24+
- [Node.js](https://nodejs.org/) 18+
- [Newman](https://www.npmjs.com/package/newman) and the TeamCity reporter:

```bash
npm install -g newman newman-reporter-teamcity
```

## Installation

```bash
git clone https://github.com/flash286/postman-load-testing.git
cd postman-load-testing
go build -o postman-load-testing .
```

## Usage

```bash
./postman-load-testing \
  -collection <collection_file_or_url> \
  -environment <environment_file_or_url> \
  -n <threads> \
  -i <iterations_per_thread> \
  -d <delay_ms> \
  -t <timeout_seconds>
```

### Arguments

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `-collection` | Yes | — | URL or path to a Postman Collection |
| `-environment` | Yes | — | URL or path to a Postman Environment |
| `-n` | No | `1` | Number of parallel threads |
| `-i` | No | `1` | Number of iterations per thread |
| `-d` | No | `0` | Delay between requests (milliseconds) |
| `-t` | No | `0` | Timeout per Newman run in seconds (0 = no timeout) |

### Example

```bash
# Run 10 parallel threads, each executing the collection 5 times
./postman-load-testing \
  -collection my-api-tests.postman_collection.json \
  -environment staging.postman_environment.json \
  -n 10 \
  -i 5
```

## Architecture

```
main.go                       CLI entry point, worker pool orchestration
├── common/                   Shared types (TestStep, WorkerSettings, Worker interface)
├── scanner/                  Parses Newman's TeamCity output into TestStep events
├── aggregator/               Aggregates TestStep events, computes stats & throughput
├── console_printer/          Styled live dashboard via lipgloss + uilive
├── logger/                   File logging (application.log, application.fail.log)
└── testdata/                 Test HTTP server & Postman collection for e2e tests
```

**Data flow:** Workers → Newman subprocess → Scanner (regex parse) → Aggregator (channel) → Console Printer

## Development

### Build

```bash
go build -o postman-load-testing .

# Cross-compile all platforms
bash make.sh
```

### Lint

```bash
# Install golangci-lint: https://golangci-lint.run/welcome/install/
golangci-lint run ./...
```

### Test

```bash
# Run all tests (requires newman)
go test -v -count=1 -timeout 120s ./...
```

The test suite includes:
- **E2E tests** — spins up a real HTTP server, runs Newman against it, validates aggregated results
- **Parallel worker tests** — 3 concurrent workers hitting the test server
- **Pipeline unit tests** — scanner → aggregator with simulated TeamCity output

## CI/CD

GitHub Actions runs on every push to `master` and on pull requests:

1. **Lint** — golangci-lint with errcheck, govet, staticcheck
2. **Test** — full test suite including real Newman e2e tests
3. **Build** — cross-compilation matrix (linux/darwin/windows × amd64/386)

## License

[Apache License 2.0](LICENSE)
