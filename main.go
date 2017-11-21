package main

import (
	"fmt"
	"os/exec"
	"context"
	"time"
	"sync"
	"github.com/olekukonko/tablewriter"
	"github.com/gosuri/uilive"
	"sort"
	"flag"
	"os"
	"postman-load-testing/common"
	"postman-load-testing/scanner"
)

var (
	newmanExecutable   = "newman"
	collectionPathCmd  = flag.String("collection", "", "URL or path to a Postman Collection")
	environmentPathCmd = flag.String("environment", "", "Specify a URL or Path to a Postman Environment")
	nParallelCmd       = flag.Int("n", 1, "Number of parallel threads")
	delayCmd           = flag.Int("d", 0, "Specify the extent of delay between requests (milliseconds) (default 0)")
	iterationCmd       = flag.Int("i", 1, "Define the number of iterations to run.")
	aggregationStream  = make(chan common.TestStep, 1000)
	requestsThroughput = 0
	stat               = make(map[string]*common.AggregatedTestStep)
	wg                 sync.WaitGroup
)

func renderResultTable(table *tablewriter.Table) {
	var keys []string

	for k := range stat {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, key := range keys {

		value := stat[key]

		datapoint := []string{
			value.Name,
			fmt.Sprintf("%v ms", value.AvgDuration),
			fmt.Sprintf("%v", value.TotalSuccess),
			fmt.Sprintf("%v", value.TotalFail),
			fmt.Sprintf("%v", value.TotalCount),
		}
		table.Append(datapoint)
		table.SetFooter([]string{"", "", "", "Requests Throughput", fmt.Sprintf("%v rps", requestsThroughput)})
	}
	table.Render()
}

func StatusPrinter(done chan string) {

	timer := time.NewTicker(time.Second * 1)
	writer := uilive.New()

	table := tablewriter.NewWriter(writer)
	table.SetHeader([]string{"Name", "Avg. Duration", "Success", "Fail", "Total"})

	writer.Start()

	for {
		select {
		case <-timer.C:
			renderResultTable(table)
			table.ClearRows()
			table.ClearFooter()
		case <-done:
			renderResultTable(table)
			writer.Stop()
			return
		}
	}
}

func aggregator(done <-chan string) {

	requestsCount := 0
	var startTime time.Time

	for {
		select {
		case msg := <-aggregationStream:

			if requestsCount == 0 {
				startTime = time.Now()
			}

			requestsCount++

			currentTime := time.Now()
			delta := currentTime.Sub(startTime)

			requestsThroughput = int(float64(requestsCount) / delta.Seconds())

			//fmt.Printf("r: %v, delta: %v, rt: %v\n", requestsCount, delta.Seconds(), requestsThroughput)

			if _, ok := stat[msg.Name]; !ok {
				stat[msg.Name] = &common.AggregatedTestStep{Name: msg.Name}
			}
			stat[msg.Name].AddStepAndRefreshStat(msg)
		case <-done:
			return
		}
	}
}

// -collection test-collection.json -environment

func worker(settings common.WorkerSettings) {

	defer wg.Done()

	ctx := context.Background()

	var newmanArgs = []string{
		"run", settings.CollectionPath,
		fmt.Sprintf("-e%s", settings.EnvironmentPath),
		"-rteamcity",
	}

	if settings.Delay > 0 {
		newmanArgs = append(newmanArgs, fmt.Sprintf("--delay-request=%v", settings.Delay))
	}

	if settings.Iterations > 0 {
		newmanArgs = append(newmanArgs, fmt.Sprintf("-n%v", settings.Iterations))
	}

	//finalCmdString := newmanExecutable + " " + strings.Join(newmanArgs[:], " ")

	//log.Println("Starting: ", finalCmdString)

	cmd := exec.CommandContext(ctx, newmanExecutable, newmanArgs...)

	stdout, _ := cmd.StdoutPipe()

	err := cmd.Start()
	if err != nil {
		panic(err);
	}

	out_scanner.OutScanner(stdout, aggregationStream)

	cmd.Wait()
}

func main() {
	flag.Parse()
	aggDone := make(chan string)

	if *collectionPathCmd == "" || *environmentPathCmd == "" {
		flag.PrintDefaults()
		os.Exit(2)
	}

	settings := common.WorkerSettings{
		Iterations:      *iterationCmd,
		Delay:           *delayCmd,
		CollectionPath:  *collectionPathCmd,
		EnvironmentPath: *environmentPathCmd,
	}

	nParallel := *nParallelCmd

	go aggregator(aggDone)
	go StatusPrinter(aggDone)

	for i := 0; i < nParallel; i ++ {
		wg.Add(1)
		go worker(settings)
	}

	wg.Wait()

	time.Sleep(time.Second * 2)

	aggDone <- "Done"
}
