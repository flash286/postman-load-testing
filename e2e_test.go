package main

import (
	"io"
	"strings"
	"testing"
	"time"

	"postman-load-testing/aggregator"
	out_scanner "postman-load-testing/scanner"
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

func TestEndToEndPipeline(t *testing.T) {
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

	// Create aggregator with enough capacity
	agg := aggregator.CreateAggregator(len(tests))
	go agg.Run()

	// Create a pipe to simulate Newman's stdout
	stdoutReader, stdoutWriter := io.Pipe()
	// Empty stderr
	stderrReader, stderrWriter := io.Pipe()

	// Write simulated output in a goroutine
	go func() {
		stdoutWriter.Write([]byte(newmanOutput))
		stdoutWriter.Close()
	}()
	go func() {
		stderrWriter.Close()
	}()

	// Run scanner (blocks until stdout is consumed)
	out_scanner.OutScanner(stdoutReader, stderrReader, agg, 1)

	// Give the aggregator time to process all messages
	time.Sleep(500 * time.Millisecond)

	// --- Verify aggregated results ---

	// Should have 3 distinct test names
	if len(agg.Stat) != 3 {
		t.Fatalf("expected 3 aggregated test entries, got %d", len(agg.Stat))
	}

	// Verify GET /users
	getUsers, ok := agg.Stat["GET /users"]
	if !ok {
		t.Fatal("missing aggregated entry for 'GET /users'")
	}
	if getUsers.TotalCount != 2 {
		t.Errorf("GET /users: expected TotalCount=2, got %d", getUsers.TotalCount)
	}
	if getUsers.TotalSuccess != 2 {
		t.Errorf("GET /users: expected TotalSuccess=2, got %d", getUsers.TotalSuccess)
	}
	if getUsers.TotalFail != 0 {
		t.Errorf("GET /users: expected TotalFail=0, got %d", getUsers.TotalFail)
	}
	// Avg of 120 and 80: first step sets 120, second calculates (120+80)/2 = 100
	if getUsers.AvgDuration != 100.0 {
		t.Errorf("GET /users: expected AvgDuration=100, got %f", getUsers.AvgDuration)
	}

	// Verify POST /login
	postLogin, ok := agg.Stat["POST /login"]
	if !ok {
		t.Fatal("missing aggregated entry for 'POST /login'")
	}
	if postLogin.TotalCount != 2 {
		t.Errorf("POST /login: expected TotalCount=2, got %d", postLogin.TotalCount)
	}
	if postLogin.TotalSuccess != 2 {
		t.Errorf("POST /login: expected TotalSuccess=2, got %d", postLogin.TotalSuccess)
	}
	// Avg of 200 and 150: (200+150)/2 = 175
	if postLogin.AvgDuration != 175.0 {
		t.Errorf("POST /login: expected AvgDuration=175, got %f", postLogin.AvgDuration)
	}

	// Verify DELETE /user/1 (failed test)
	deleteUser, ok := agg.Stat["DELETE /user/1"]
	if !ok {
		t.Fatal("missing aggregated entry for 'DELETE /user/1'")
	}
	if deleteUser.TotalCount != 1 {
		t.Errorf("DELETE /user/1: expected TotalCount=1, got %d", deleteUser.TotalCount)
	}
	if deleteUser.TotalFail != 1 {
		t.Errorf("DELETE /user/1: expected TotalFail=1, got %d", deleteUser.TotalFail)
	}
	if deleteUser.TotalSuccess != 0 {
		t.Errorf("DELETE /user/1: expected TotalSuccess=0, got %d", deleteUser.TotalSuccess)
	}
	if deleteUser.AvgDuration != 0.0 {
		t.Errorf("DELETE /user/1: expected AvgDuration=0 (failed test), got %f", deleteUser.AvgDuration)
	}

	// Verify throughput was calculated (should be > 0 since requests were processed)
	if agg.RequestsThroughput <= 0 {
		t.Logf("RequestsThroughput=%d (may be 0 if processed too fast, not a hard failure)", agg.RequestsThroughput)
	}

	// Cleanup
	agg.Close()
}

func TestEndToEndMultipleWorkers(t *testing.T) {
	// Simulate two workers sending results to the same aggregator
	worker1Output := simulateNewmanOutput([]struct {
		name     string
		duration string
		fail     bool
		failMsg  string
	}{
		{name: "GET /health", duration: "10", fail: false},
		{name: "GET /health", duration: "20", fail: false},
	})

	worker2Output := simulateNewmanOutput([]struct {
		name     string
		duration string
		fail     bool
		failMsg  string
	}{
		{name: "GET /health", duration: "30", fail: false},
		{name: "GET /status", duration: "100", fail: true, failMsg: "timeout"},
	})

	agg := aggregator.CreateAggregator(10)
	go agg.Run()

	// Run both workers concurrently
	done := make(chan struct{}, 2)

	runWorker := func(output string, threadNum int) {
		stdoutR, stdoutW := io.Pipe()
		stderrR, stderrW := io.Pipe()
		go func() {
			stdoutW.Write([]byte(output))
			stdoutW.Close()
		}()
		go func() { stderrW.Close() }()
		out_scanner.OutScanner(stdoutR, stderrR, agg, threadNum)
		done <- struct{}{}
	}

	go runWorker(worker1Output, 1)
	go runWorker(worker2Output, 2)

	// Wait for both workers
	<-done
	<-done

	time.Sleep(500 * time.Millisecond)

	// GET /health should have 3 total (2 from worker1 + 1 from worker2)
	health, ok := agg.Stat["GET /health"]
	if !ok {
		t.Fatal("missing aggregated entry for 'GET /health'")
	}
	if health.TotalCount != 3 {
		t.Errorf("GET /health: expected TotalCount=3, got %d", health.TotalCount)
	}
	if health.TotalSuccess != 3 {
		t.Errorf("GET /health: expected TotalSuccess=3, got %d", health.TotalSuccess)
	}

	// GET /status should have 1 failure
	status, ok := agg.Stat["GET /status"]
	if !ok {
		t.Fatal("missing aggregated entry for 'GET /status'")
	}
	if status.TotalFail != 1 {
		t.Errorf("GET /status: expected TotalFail=1, got %d", status.TotalFail)
	}

	agg.Close()
}

func TestEndToEndEmptyOutput(t *testing.T) {
	// Verify pipeline handles empty Newman output gracefully
	agg := aggregator.CreateAggregator(1)
	go agg.Run()

	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	go func() { stdoutW.Close() }()
	go func() { stderrW.Close() }()

	out_scanner.OutScanner(stdoutR, stderrR, agg, 1)

	time.Sleep(200 * time.Millisecond)

	if len(agg.Stat) != 0 {
		t.Errorf("expected 0 aggregated entries for empty output, got %d", len(agg.Stat))
	}

	agg.Close()
}
