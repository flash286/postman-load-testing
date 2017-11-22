package common

import (
	"time"
	"fmt"
)

const TestStatusSuccess = "success"
const TestStatusFail = "fail"

type Worker interface {
	Close()
	Run()
}

type WorkerSettings struct {
	CollectionPath  string
	EnvironmentPath string
	Delay           int
	Iterations      int
}

type TestStep struct {
	Name         string
	Status       string
	Duration     int
	Message      string
	StartTime    time.Time
	ThreadNumber int
}

type AggregatedTestStep struct {
	Name         string
	TotalCount   int
	TotalSuccess int
	TotalFail    int
	AvgDuration  float64
	Steps        []TestStep
}

func (ts *TestStep) String() string {
	return fmt.Sprintf("\t<TestStep: Name: %s, Duration: %d, Status: %s>,\n", ts.Name, ts.Duration, ts.Status)
}

func (testStat *AggregatedTestStep) String() string {
	return fmt.Sprintf(
		"\n<AggregatedTestStep\n\tName: %s\n\tTotalCount: %v\n\tSuccess/Fail: %v\\%v\n\tAvg Duration: %v\n>",
		testStat.Name, testStat.TotalCount, testStat.TotalSuccess, testStat.TotalFail, testStat.AvgDuration,
	)
}

func (testStat *AggregatedTestStep) AddStepAndRefreshStat(step TestStep) {
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
