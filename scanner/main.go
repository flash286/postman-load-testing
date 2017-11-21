package out_scanner

import (
	"io"
	"bufio"
	"time"
	"strconv"
	"regexp"
	"postman-load-testing/common"
	"postman-load-testing/logger"
	"fmt"
)

var (
	taskStartRe    = regexp.MustCompile(`^##teamcity\[testStarted name='(?P<TaskName>.*)' captureStandardOutput='(?P<TaskOutPut>.*)']`)
	taskFinishedRe = regexp.MustCompile(`##teamcity\[testFinished name='(?P<TaskName>.*)' duration='(?P<TaskOutPut>.*)']`)
	taskFailedRe   = regexp.MustCompile(`##teamcity\[testFailed name='(?P<TaskName>.*)' message='(?P<TaskOutPut>.*)']`)
)

func LogFailMsg(msg *common.TestStep) {
	msgBody := fmt.Sprintf("TestName: %v, TestMsg: %v, Duration: %v", msg.Name, msg.Message, msg.Duration)
	logger.FailLog.Printf("Thread[%v]: %s", msg.ThreadNumber, msgBody)
}

func OutScanner(stdout io.ReadCloser, stderr io.ReadCloser, aggregationStream chan<- common.TestStep, threadNumber int) {

	tasks := make(map[string]*common.TestStep)

	scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		msg := scanner.Text()

		logger.Log.Printf("Thread[%v]: %s", threadNumber, msg)

		testStartedMeta := taskStartRe.FindAllStringSubmatch(msg, -1)
		testFinishedMeta := taskFinishedRe.FindAllStringSubmatch(msg, -1)
		testFailedMeta := taskFailedRe.FindAllStringSubmatch(msg, -1)

		if len(testStartedMeta) > 0 {
			taskName := testStartedMeta[0][1]
			tasks[taskName] = &common.TestStep{Name: taskName, StartTime: time.Now(), ThreadNumber: threadNumber}
		} else if len(testFinishedMeta) > 0 {
			taskName := testFinishedMeta[0][1]
			taskDuration := testFinishedMeta[0][2]

			if val, ok := tasks[taskName]; ok {
				duration, _ := strconv.Atoi(taskDuration)
				val.Duration = duration

				if val.Status != common.TestStatusFail {
					val.Status = common.TestStatusSuccess
				}

				tasks[taskName] = val

				aggregationStream <- *tasks[taskName]
			}
		} else if len(testFailedMeta) > 0 {
			taskName := testFailedMeta[0][1]
			taskMessage := testFailedMeta[0][2]

			if val, ok := tasks[taskName]; ok {
				val.Duration = 0
				val.Message = taskMessage
				val.Status = common.TestStatusFail
				tasks[taskName] = val
			}
		}
	}
}
