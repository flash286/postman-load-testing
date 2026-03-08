package aggregator

import (
	"postman-load-testing/common"
	"sync"
	"time"
)

type Aggregator struct {
	Source             chan common.TestStep
	Quit               chan bool
	mu                 sync.RWMutex
	requestsThroughput int
	requestsCount      int
	stat               map[string]*common.AggregatedTestStep
}

func (w *Aggregator) Close() {
	w.Quit <- true
}

func (w *Aggregator) GetStat() map[string]*common.AggregatedTestStep {
	w.mu.RLock()
	defer w.mu.RUnlock()
	// Return a shallow copy of the map to avoid concurrent map access
	result := make(map[string]*common.AggregatedTestStep, len(w.stat))
	for k, v := range w.stat {
		cpy := *v
		result[k] = &cpy
	}
	return result
}

func (w *Aggregator) GetThroughput() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.requestsThroughput
}

func (w *Aggregator) Run() {
	w.requestsCount = 0
	var startTime time.Time
	for {
		select {
		case msg := <-w.Source:
			if w.requestsCount == 0 {
				startTime = time.Now()
			}

			w.requestsCount++

			currentTime := time.Now()
			delta := currentTime.Sub(startTime)

			w.mu.Lock()
			if delta.Seconds() > 0 {
				w.requestsThroughput = int(float64(w.requestsCount) / delta.Seconds())
			}

			if _, ok := w.stat[msg.Name]; !ok {
				w.stat[msg.Name] = &common.AggregatedTestStep{Name: msg.Name}
			}
			w.stat[msg.Name].AddStepAndRefreshStat(msg)
			w.mu.Unlock()
		case <-w.Quit:
			return
		}
	}
}

func CreateAggregator(capacity int) *Aggregator {
	aggregator := Aggregator{
		Source: make(chan common.TestStep, capacity),
		Quit:   make(chan bool),
		stat:   make(map[string]*common.AggregatedTestStep),
	}
	return &aggregator
}
