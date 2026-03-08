# PostmanLoadTester

[![CI](https://github.com/flash286/postman-load-testing/actions/workflows/ci.yml/badge.svg)](https://github.com/flash286/postman-load-testing/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Newman](https://img.shields.io/badge/Newman-6.x-FF6C37?logo=postman&logoColor=white)](https://www.npmjs.com/package/newman)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

A Go-based load testing tool that runs [Postman](https://www.postman.com/) collections in parallel using [Newman](https://github.com/postmanlabs/newman). Spawn multiple concurrent workers, aggregate results in real time, and view live statistics in your terminal.

```
+-------------------+-----------------+---------+------+-------+
|       NAME        |  AVG. DURATION  | SUCCESS | FAIL | TOTAL |
+-------------------+-----------------+---------+------+-------+
| Health Check      | 12 ms           |      30 |    0 |    30 |
| Get Users         | 45 ms           |      30 |    0 |    30 |
| Login             | 23 ms           |      28 |    2 |    30 |
+-------------------+-----------------+---------+------+-------+
|                   REQUESTS THROUGHPUT  |         42 RPS       |
+-------------------+-----------------+---------+------+-------+
```

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
  -d <delay_ms>
```

### Arguments

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `-collection` | Yes | — | URL or path to a Postman Collection |
| `-environment` | Yes | — | URL or path to a Postman Environment |
| `-n` | No | `1` | Number of parallel threads |
| `-i` | No | `1` | Number of iterations per thread |
| `-d` | No | `0` | Delay between requests (milliseconds) |

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
├── console_printer/          Live ASCII table rendering via uilive
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
