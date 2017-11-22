package main

import (
	"fmt"
	"os/exec"
	"context"
	"sync"
	"flag"
	"os"
	"postman-load-testing/common"
	"postman-load-testing/scanner"
	"postman-load-testing/logger"
	"strings"
	"postman-load-testing/aggregator"
	"postman-load-testing/console_printer"
	"time"
)

var (
	newmanExecutable   = "newman"
	collectionPathCmd  = flag.String("collection", "", "URL or path to a Postman Collection")
	environmentPathCmd = flag.String("environment", "", "Specify a URL or Path to a Postman Environment")
	nParallelCmd       = flag.Int("n", 1, "Number of parallel threads")
	delayCmd           = flag.Int("d", 0, "Specify the extent of delay between requests (milliseconds) (default 0)")
	iterationCmd       = flag.Int("i", 1, "Define the number of iterations to run.")
	wg                 sync.WaitGroup
)

// -collection test-collection.json -environment

func worker(settings common.WorkerSettings, aggregatorWorker *aggregator.Aggregator, threadNumber int) {

	defer wg.Done()

	ctx := context.Background()

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

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	err := cmd.Start()
	if err != nil {
		panic(err);
	}

	out_scanner.OutScanner(stdout, stderr, aggregatorWorker, threadNumber)

	cmd.Wait()
}

func main() {

	flag.Parse()

	if *collectionPathCmd == "" || *environmentPathCmd == "" {
		flag.PrintDefaults()
		os.Exit(2)
	}

	fmt.Printf("Log File: %s\n", logger.LogPath)
	fmt.Printf("Fail Log File: %s\n", logger.FailLogPath)

	settings := common.WorkerSettings{
		Iterations:      *iterationCmd,
		Delay:           *delayCmd,
		CollectionPath:  *collectionPathCmd,
		EnvironmentPath: *environmentPathCmd,
	}
	nParallel := *nParallelCmd

	aggregatorWorker := aggregator.CreateAggregator(nParallel * settings.Iterations)
	consoleStatusWorker := console_printer.CreateConsoleStatusPrinter(aggregatorWorker)

	for i := 0; i < nParallel; i ++ {
		wg.Add(1)
		go worker(settings, aggregatorWorker, i+1)
	}

	go aggregatorWorker.Run()
	go consoleStatusWorker.Run()

	wg.Wait()

	time.Sleep(time.Second * 2)

	aggregatorWorker.Close()
	consoleStatusWorker.Close()

	time.Sleep(time.Second * 2)
}
