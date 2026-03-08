package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"

	"postman-load-testing/aggregator"
	"postman-load-testing/common"
	"postman-load-testing/console_printer"
	"postman-load-testing/logger"
	out_scanner "postman-load-testing/scanner"
)

var (
	newmanExecutable   = "newman"
	collectionPathCmd  = flag.String("collection", "", "URL or path to a Postman Collection")
	environmentPathCmd = flag.String("environment", "", "Specify a URL or Path to a Postman Environment")
	nParallelCmd       = flag.Int("n", 1, "Number of parallel threads")
	delayCmd           = flag.Int("d", 0, "Specify the extent of delay between requests (milliseconds) (default 0)")
	iterationCmd       = flag.Int("i", 1, "Define the number of iterations to run.")
	timeoutCmd         = flag.Int("t", 0, "Timeout for each Newman run in seconds (0 = no timeout)")
	wg                 sync.WaitGroup
)

func worker(settings common.WorkerSettings, aggregatorWorker *aggregator.Aggregator, threadNumber int, timeout int) {
	defer wg.Done()

	var ctx context.Context
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	var newmanArgs = []string{
		"run", settings.CollectionPath,
		fmt.Sprintf("-e%s", settings.EnvironmentPath),
		"-rteamcity,cli",
	}

	if settings.Delay > 0 {
		newmanArgs = append(newmanArgs, fmt.Sprintf("--delay-request=%v", settings.Delay))
	}

	if settings.Iterations > 0 {
		newmanArgs = append(newmanArgs, fmt.Sprintf("-n%v", settings.Iterations))
	}

	finalCmdString := newmanExecutable + " " + strings.Join(newmanArgs[:], " ")
	logger.Log.Printf("Thread[%v]: Starting: %v", threadNumber, finalCmdString)

	cmd := exec.CommandContext(ctx, newmanExecutable, newmanArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Log.Printf("Thread[%v]: failed to get stdout pipe: %v", threadNumber, err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		logger.Log.Printf("Thread[%v]: failed to get stderr pipe: %v", threadNumber, err)
		return
	}

	if err := cmd.Start(); err != nil {
		logger.Log.Printf("Thread[%v]: failed to start newman: %v", threadNumber, err)
		return
	}

	out_scanner.OutScanner(stdout, stderr, aggregatorWorker, threadNumber)

	if err := cmd.Wait(); err != nil {
		logger.Log.Printf("Thread[%v]: newman exited with error: %v", threadNumber, err)
	}
}

func main() {
	flag.Parse()

	if *collectionPathCmd == "" || *environmentPathCmd == "" {
		flag.PrintDefaults()
		os.Exit(2)
	}

	if *nParallelCmd <= 0 {
		fmt.Fprintln(os.Stderr, "error: -n must be a positive integer")
		os.Exit(2)
	}
	if *iterationCmd <= 0 {
		fmt.Fprintln(os.Stderr, "error: -i must be a positive integer")
		os.Exit(2)
	}
	if *delayCmd < 0 {
		fmt.Fprintln(os.Stderr, "error: -d must be non-negative")
		os.Exit(2)
	}

	settings := common.WorkerSettings{
		Iterations:      *iterationCmd,
		Delay:           *delayCmd,
		CollectionPath:  *collectionPathCmd,
		EnvironmentPath: *environmentPathCmd,
	}
	nParallel := *nParallelCmd

	printBanner(settings, nParallel)

	aggregatorWorker := aggregator.CreateAggregator(nParallel * settings.Iterations)
	consoleStatusWorker := console_printer.CreateConsoleStatusPrinter(aggregatorWorker, nParallel)

	go aggregatorWorker.Run()
	go consoleStatusWorker.Run()

	for i := 0; i < nParallel; i++ {
		wg.Add(1)
		go worker(settings, aggregatorWorker, i+1, *timeoutCmd)
	}

	wg.Wait()

	// Allow aggregator to finish processing remaining messages
	time.Sleep(500 * time.Millisecond)

	aggregatorWorker.Close()
	consoleStatusWorker.Close()
}

func printBanner(settings common.WorkerSettings, nParallel int) {
	cyan := lipgloss.Color("#00D7FF")
	gray := lipgloss.Color("#6C6C6C")

	logo := lipgloss.NewStyle().Bold(true).Foreground(cyan).Render(
		"POSTMAN LOAD TESTER")

	labelStyle := lipgloss.NewStyle().Foreground(gray)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)

	info := fmt.Sprintf(
		"  %s %s\n  %s %s\n  %s %s   %s %s   %s %s",
		labelStyle.Render("Collection:"),
		valueStyle.Render(settings.CollectionPath),
		labelStyle.Render("Environment:"),
		valueStyle.Render(settings.EnvironmentPath),
		labelStyle.Render("Workers:"),
		valueStyle.Render(fmt.Sprintf("%d", nParallel)),
		labelStyle.Render("Iterations:"),
		valueStyle.Render(fmt.Sprintf("%d", settings.Iterations)),
		labelStyle.Render("Delay:"),
		valueStyle.Render(fmt.Sprintf("%dms", settings.Delay)),
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cyan).
		PaddingLeft(1).
		PaddingRight(1).
		Render(fmt.Sprintf(" %s\n\n%s\n\n  %s %s  |  %s %s",
			logo, info,
			labelStyle.Render("Log:"), valueStyle.Render(logger.LogPath),
			labelStyle.Render("Fail Log:"), valueStyle.Render(logger.FailLogPath),
		))

	fmt.Println(box)
	fmt.Println()
}
