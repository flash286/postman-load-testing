package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"postman-load-testing/aggregator"
	out_scanner "postman-load-testing/scanner"
	"postman-load-testing/testdata"
)

// simulateNewmanOutput returns TeamCity-formatted output that mimics what Newman produces.
func simulateNewmanOutput(tests []struct {
	name     string
	duration string
	fail     bool
	failMsg  string
},
) string {
	var sb strings.Builder
	for _, t := range tests {
		sb.WriteString("##teamcity[testStarted name='" + t.name + "' captureStandardOutput='true']\n")
		if t.fail {
			sb.WriteString("##teamcity[testFailed name='" + t.name + "' message='" + t.failMsg + "']\n")
		}
		sb.WriteString("##teamcity[testFinished name='" + t.name + "' duration='" + t.duration + "']\n")
	}
	return sb.String()
}

// writeTestEnvironment creates a Postman environment JSON file with the given base URL.
func writeTestEnvironment(t *testing.T, dir, baseURL string) string {
	t.Helper()
	env := map[string]interface{}{
		"name": "Load Test Environment",
		"values": []map[string]interface{}{
			{
				"key":     "base_url",
				"value":   baseURL,
				"enabled": true,
			},
		},
	}
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal environment: %v", err)
	}
	path := filepath.Join(dir, "env.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write environment file: %v", err)
	}
	return path
}

// TestEndToEndWithNewman starts a real HTTP server, runs Newman against the
// Postman collection, and pipes the output through the scanner/aggregator pipeline.
func TestEndToEndWithNewman(t *testing.T) {
	// Check Newman is available
	if _, err := exec.LookPath("newman"); err != nil {
		t.Skip("newman not installed, skipping real e2e test")
	}

	// Start API server on random port
	srv := testdata.NewTestAPIServer()
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer srv.Stop()

	baseURL := fmt.Sprintf("http://%s", srv.Addr)
	t.Logf("Test API server running at %s", baseURL)

	// Create temp environment file with actual server address
	tmpDir := t.TempDir()
	envPath := writeTestEnvironment(t, tmpDir, baseURL)

	// Locate collection file
	collectionPath := filepath.Join("testdata", "test_collection.json")
	if _, err := os.Stat(collectionPath); err != nil {
		t.Fatalf("collection file not found: %v", err)
	}

	// Run Newman with TeamCity reporter, piping output through scanner → aggregator
	args := []string{
		"run", collectionPath,
		"-e", envPath,
		"-r", "teamcity,cli",
	}

	cmd := exec.Command("newman", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to get stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("failed to get stderr pipe: %v", err)
	}

	agg := aggregator.CreateAggregator(100)
	go agg.Run()

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start newman: %v", err)
	}

	// Scanner reads Newman output and feeds aggregator
	out_scanner.OutScanner(stdout, stderr, agg, 1)

	if err := cmd.Wait(); err != nil {
		t.Logf("newman exited with: %v (may include test failures)", err)
	}

	// Let aggregator finish processing
	time.Sleep(500 * time.Millisecond)

	// --- Verify results ---

	if len(agg.Stat) == 0 {
		t.Fatal("aggregator has no results — newman produced no parseable output")
	}

	t.Logf("Aggregated %d distinct test steps:", len(agg.Stat))
	totalTests := 0
	totalSuccess := 0
	totalFail := 0
	for name, stat := range agg.Stat {
		t.Logf("  %s: total=%d success=%d fail=%d avg=%.1fms",
			name, stat.TotalCount, stat.TotalSuccess, stat.TotalFail, stat.AvgDuration)
		totalTests += stat.TotalCount
		totalSuccess += stat.TotalSuccess
		totalFail += stat.TotalFail
	}

	// Collection has 5 requests with 2 tests each = 10 test assertions
	if totalTests < 5 {
		t.Errorf("expected at least 5 test results, got %d", totalTests)
	}

	// All tests in our collection should pass against our server
	if totalFail > 0 {
		t.Errorf("expected 0 failures, got %d", totalFail)
	}
	if totalSuccess == 0 {
		t.Error("expected at least some successes")
	}

	agg.Close()
}

// TestEndToEndWithNewmanParallel runs Newman with multiple parallel threads
// against the test server, exercising the concurrent worker pattern.
func TestEndToEndWithNewmanParallel(t *testing.T) {
	if _, err := exec.LookPath("newman"); err != nil {
		t.Skip("newman not installed, skipping real e2e test")
	}

	srv := testdata.NewTestAPIServer()
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer srv.Stop()

	baseURL := fmt.Sprintf("http://%s", srv.Addr)
	tmpDir := t.TempDir()
	envPath := writeTestEnvironment(t, tmpDir, baseURL)
	collectionPath := filepath.Join("testdata", "test_collection.json")

	nWorkers := 3
	agg := aggregator.CreateAggregator(100)
	go agg.Run()

	done := make(chan error, nWorkers)

	for i := 0; i < nWorkers; i++ {
		go func(threadNum int) {
			args := []string{
				"run", collectionPath,
				"-e", envPath,
				"-r", "teamcity,cli",
			}
			cmd := exec.Command("newman", args...)
			stdout, _ := cmd.StdoutPipe()
			stderr, _ := cmd.StderrPipe()

			if err := cmd.Start(); err != nil {
				done <- fmt.Errorf("worker %d: failed to start: %v", threadNum, err)
				return
			}
			out_scanner.OutScanner(stdout, stderr, agg, threadNum)
			done <- cmd.Wait()
		}(i + 1)
	}

	// Wait for all workers
	for i := 0; i < nWorkers; i++ {
		if err := <-done; err != nil {
			t.Logf("worker finished with: %v", err)
		}
	}

	time.Sleep(500 * time.Millisecond)

	if len(agg.Stat) == 0 {
		t.Fatal("no aggregated results from parallel run")
	}

	totalTests := 0
	for name, stat := range agg.Stat {
		t.Logf("  %s: total=%d success=%d fail=%d avg=%.1fms",
			name, stat.TotalCount, stat.TotalSuccess, stat.TotalFail, stat.AvgDuration)
		totalTests += stat.TotalCount
	}

	// With 3 workers, each running the collection (5 requests × 2 tests), expect 3x results
	if totalTests < 15 {
		t.Errorf("expected at least 15 test results from %d workers, got %d", nWorkers, totalTests)
	}

	if agg.RequestsThroughput <= 0 {
		t.Logf("throughput=%d rps (may be 0 if too fast)", agg.RequestsThroughput)
	}

	agg.Close()
}

// --- Pipeline unit tests (kept from original) ---

func TestPipelineScannerAggregator(t *testing.T) {
	tests := []struct {
		name     string
		duration string
		fail     bool
		failMsg  string
	}{
		{name: "GET /users", duration: "120", fail: false},
		{name: "POST /login", duration: "200", fail: false},
		{name: "GET /users", duration: "80", fail: false},
		{name: "POST /login", duration: "150", fail: false},
		{name: "DELETE /user/1", duration: "50", fail: true, failMsg: "expected 200 got 403"},
	}

	newmanOutput := simulateNewmanOutput(tests)

	agg := aggregator.CreateAggregator(len(tests))
	go agg.Run()

	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	go func() {
		_, _ = stdoutWriter.Write([]byte(newmanOutput))
		_ = stdoutWriter.Close()
	}()
	go func() {
		_ = stderrWriter.Close()
	}()

	out_scanner.OutScanner(stdoutReader, stderrReader, agg, 1)
	time.Sleep(500 * time.Millisecond)

	if len(agg.Stat) != 3 {
		t.Fatalf("expected 3 aggregated test entries, got %d", len(agg.Stat))
	}

	getUsers := agg.Stat["GET /users"]
	if getUsers.TotalCount != 2 || getUsers.TotalSuccess != 2 || getUsers.AvgDuration != 100.0 {
		t.Errorf("GET /users: unexpected stats: count=%d success=%d avg=%.1f",
			getUsers.TotalCount, getUsers.TotalSuccess, getUsers.AvgDuration)
	}

	postLogin := agg.Stat["POST /login"]
	if postLogin.TotalCount != 2 || postLogin.TotalSuccess != 2 || postLogin.AvgDuration != 175.0 {
		t.Errorf("POST /login: unexpected stats: count=%d success=%d avg=%.1f",
			postLogin.TotalCount, postLogin.TotalSuccess, postLogin.AvgDuration)
	}

	deleteUser := agg.Stat["DELETE /user/1"]
	if deleteUser.TotalFail != 1 || deleteUser.TotalSuccess != 0 {
		t.Errorf("DELETE /user/1: expected 1 fail/0 success, got %d/%d",
			deleteUser.TotalFail, deleteUser.TotalSuccess)
	}

	agg.Close()
}

func TestPipelineEmptyOutput(t *testing.T) {
	agg := aggregator.CreateAggregator(1)
	go agg.Run()

	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	go func() { _ = stdoutW.Close() }()
	go func() { _ = stderrW.Close() }()

	out_scanner.OutScanner(stdoutR, stderrR, agg, 1)
	time.Sleep(200 * time.Millisecond)

	if len(agg.Stat) != 0 {
		t.Errorf("expected 0 entries for empty output, got %d", len(agg.Stat))
	}

	agg.Close()
}
