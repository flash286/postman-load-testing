package main

import (
	"fmt"
	"os/exec"
	"context"
	"bufio"
	"io"
	"regexp"
	"strconv"
	"log"
	"time"
	"sync"
	"strings"
	"github.com/olekukonko/tablewriter"
	"github.com/gosuri/uilive"
	"sort"
)

const TestStatusSuccess = "success"
const TestStatusFail = "fail"

type WorkerSettings struct {
	Delay      int
	Iterations int
}

type TestStep struct {
	Name      string
	Status    string
	Duration  int
	Message   string
	StartTime time.Time
}

type AggregatedTestStep struct {
	Name         string
	TotalCount   int
	TotalSuccess int
	TotalFail    int
	AvgDuration  float64
	Steps        []TestStep
}

var (
	newmanExecutable  = "newman"
	collectionPath    = "c33c0e4cee3b90533b2a.json"
	environmentPath   = "Local.postman_environment.json"
	nParallel         = 1
	aggregationStream = make(chan TestStep)
	stat              = make(map[string]*AggregatedTestStep)
	wg                sync.WaitGroup
)

func (ts *TestStep) String() string {
	return fmt.Sprintf("\t<TestStep: Name: %s, Duration: %d, Status: %s>,\n", ts.Name, ts.Duration, ts.Status)
}

func (testStat *AggregatedTestStep) String() string {
	return fmt.Sprintf(
		"\n<AggregatedTestStep\n\tName: %s\n\tTotalCount: %v\n\tSuccess/Fail: %v\\%v\n\tAvg Duration: %v\n>",
		testStat.Name, testStat.TotalCount, testStat.TotalSuccess, testStat.TotalFail, testStat.AvgDuration,
	)
}

func (testStat *AggregatedTestStep) addStepAndRefreshStat(step TestStep) {
	testStat.Steps = append(testStat.Steps, step)

	if step.Status == TestStatusSuccess {
		testStat.TotalSuccess++
	} else if step.Status == TestStatusFail {
		testStat.TotalFail++
	}
	testStat.TotalCount++

	if step.Status == TestStatusSuccess {
		if testStat.AvgDuration == 0.0 {
			testStat.AvgDuration = float64(step.Duration)
		} else {
			newAvg := (testStat.AvgDuration + float64(step.Duration)) / 2
			testStat.AvgDuration = newAvg
		}
	}
}

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
		case <-done:
			renderResultTable(table)
			writer.Stop()
			return
		}
	}
}

func aggregator(done chan string) {
	for {
		select {
		case msg := <-aggregationStream:
			if _, ok := stat[msg.Name]; !ok {
				stat[msg.Name] = &AggregatedTestStep{Name: msg.Name}
				//stat[msg.Name].Steps = make([]TestStep, 10)
			}
			stat[msg.Name].addStepAndRefreshStat(msg)
			//fmt.Println(stat[msg.Name])
		case <-done:
			return
		}
	}
}

func worker(settings WorkerSettings) {

	//defer wg.Done()

	ctx := context.Background()

	var newmanArgs = []string{"run", collectionPath, fmt.Sprintf("-e%s", environmentPath), "-rteamcity"}

	if settings.Delay > 0 {
		newmanArgs = append(newmanArgs, fmt.Sprintf("--delay-request=%v", settings.Delay))
	}

	if settings.Iterations > 0 {
		newmanArgs = append(newmanArgs, fmt.Sprintf("-n%v", settings.Iterations))
	}

	finalCmdString := newmanExecutable + " " + strings.Join(newmanArgs[:], " ")

	log.Println("Starting: ", finalCmdString)

	cmd := exec.CommandContext(ctx, newmanExecutable, newmanArgs...)

	stdout, _ := cmd.StdoutPipe()

	err := cmd.Start()
	if err != nil {
		panic(err);
	}

	OutScanner(stdout)
	cmd.Wait()
	wg.Done()
}

func OutScanner(stdout io.ReadCloser) {

	tasks := make(map[string]*TestStep)
	taskStartRe := regexp.MustCompile(`^##teamcity\[testStarted name='(?P<TaskName>.*)' captureStandardOutput='(?P<TaskOutPut>.*)']`)
	taskFinishedRe := regexp.MustCompile(`##teamcity\[testFinished name='(?P<TaskName>.*)' duration='(?P<TaskOutPut>.*)']`)
	taskFailedRe := regexp.MustCompile(`##teamcity\[testFailed name='(?P<TaskName>.*)' message='(?P<TaskOutPut>.*)']`)

	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		msg := scanner.Text()
		testStartedMeta := taskStartRe.FindAllStringSubmatch(msg, -1)
		testFinishedMeta := taskFinishedRe.FindAllStringSubmatch(msg, -1)
		testFailedMeta := taskFailedRe.FindAllStringSubmatch(msg, -1)

		//fmt.Printf("%s\n", msg)

		if len(testStartedMeta) > 0 {
			taskName := testStartedMeta[0][1]
			tasks[taskName] = &TestStep{Name: taskName, StartTime: time.Now()}
		} else if len(testFinishedMeta) > 0 {
			taskName := testFinishedMeta[0][1]
			taskDuration := testFinishedMeta[0][2]

			if val, ok := tasks[taskName]; ok {
				duration, _ := strconv.Atoi(taskDuration)
				val.Duration = duration

				if val.Status != TestStatusFail {
					val.Status = TestStatusSuccess
				}
				aggregationStream <- *tasks[taskName]
			}
		} else if len(testFailedMeta) > 0 {
			taskName := testFailedMeta[0][1]
			taskMessage := testFailedMeta[0][2]

			if val, ok := tasks[taskName]; ok {
				val.Duration = 0
				val.Message = taskMessage
				val.Status = TestStatusFail
			}
			aggregationStream <- *tasks[taskName]
		}
	}
}

func main() {
	//timeStart := time.Now()
	done := make(chan string)
	aggDone := make(chan string)

	settings := WorkerSettings{Iterations: 10, Delay: 1000}

	go aggregator(aggDone)
	go StatusPrinter(aggDone)

	for i := 0; i < nParallel; i ++ {
		wg.Add(1)
		go worker(settings)
	}

	wg.Wait()

	aggDone <- "Done"

	//timeFinish := time.Now()
	//duration := timeFinish.Sub(timeStart)

	//log.Printf("Time: %v seconds", duration.Seconds())
}
