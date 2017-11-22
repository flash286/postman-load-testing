package aggregator

import (
	"postman-load-testing/common"
	"time"
)

type Aggregator struct {
	Source             chan common.TestStep
	Quit               chan bool
	RequestsThroughput int
	requestsCount      int
	Stat               map[string]*common.AggregatedTestStep
}

func (w *Aggregator) Close() {
	w.Quit <- true
}

func (w *Aggregator) Run() {
	w.requestsCount = 0
	var startTime time.Time
	//q := false
	for {
		select {
		case msg := <-w.Source:
			if w.requestsCount == 0 {
				startTime = time.Now()
			}

			w.requestsCount++

			currentTime := time.Now()
			delta := currentTime.Sub(startTime)

			w.RequestsThroughput = int(float64(w.requestsCount) / delta.Seconds())

			if _, ok := w.Stat[msg.Name]; !ok {
				w.Stat[msg.Name] = &common.AggregatedTestStep{Name: msg.Name}
			}
			w.Stat[msg.Name].AddStepAndRefreshStat(msg)
		case <-w.Quit:
			return
		}
	}
}

func CreateAggregator(capacity int) *Aggregator {
	aggregator := Aggregator{
		Source: make(chan common.TestStep, capacity),
		Quit:   make(chan bool),
		Stat:   make(map[string]*common.AggregatedTestStep),
	}
	return &aggregator
}
