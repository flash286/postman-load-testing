package out_scanner

import (
	"io"
	"bufio"
	"time"
	"strconv"
	"regexp"
	"postman-load-testing/common"
)


var (
	taskStartRe        = regexp.MustCompile(`^##teamcity\[testStarted name='(?P<TaskName>.*)' captureStandardOutput='(?P<TaskOutPut>.*)']`)
	taskFinishedRe     = regexp.MustCompile(`##teamcity\[testFinished name='(?P<TaskName>.*)' duration='(?P<TaskOutPut>.*)']`)
	taskFailedRe       = regexp.MustCompile(`##teamcity\[testFailed name='(?P<TaskName>.*)' message='(?P<TaskOutPut>.*)']`)
)

func OutScanner(stdout io.ReadCloser, aggregationStream chan common.TestStep) {

	tasks := make(map[string]*common.TestStep)

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
			tasks[taskName] = &common.TestStep{Name: taskName, StartTime: time.Now()}
		} else if len(testFinishedMeta) > 0 {
			taskName := testFinishedMeta[0][1]
			taskDuration := testFinishedMeta[0][2]

			if val, ok := tasks[taskName]; ok {
				duration, _ := strconv.Atoi(taskDuration)
				val.Duration = duration

				if val.Status != common.TestStatusFail {
					val.Status = common.TestStatusSuccess
				}
				aggregationStream <- *tasks[taskName]
			}
		} else if len(testFailedMeta) > 0 {
			taskName := testFailedMeta[0][1]
			taskMessage := testFailedMeta[0][2]

			if val, ok := tasks[taskName]; ok {
				val.Duration = 0
				val.Message = taskMessage
				val.Status = common.TestStatusFail
			}
			aggregationStream <- *tasks[taskName]
		}
	}
}
