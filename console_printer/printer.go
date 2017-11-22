package console_printer

import (
	"github.com/gosuri/uilive"
	"fmt"
	"bytes"
	"postman-load-testing/aggregator"
	"github.com/olekukonko/tablewriter"
	"sort"
	"io"
	"postman-load-testing/common"
	"postman-load-testing/logger"
	"time"
)

type ConsoleStatusPrinter struct {
	Quit            chan bool
	aggregateWorker *aggregator.Aggregator
}

func (p *ConsoleStatusPrinter) Close() {
	p.Quit <- true
}

func (p *ConsoleStatusPrinter) Run() {

	timer := time.NewTicker(time.Second * 1)
	writer := uilive.New()
	table := CreateStatTable(writer)
	writer.Start()

	for {
		select {
		case <-timer.C:
			renderResultTable(table, p.aggregateWorker.Stat, p.aggregateWorker.RequestsThroughput)
			table.ClearRows()
			table.ClearFooter()
		case <-p.Quit:
			writer.Stop()
			buffer := bytes.NewBufferString("\n")
			finalTable := CreateStatTable(buffer)
			finalTable.SetCaption(true,"Final Results")
			renderResultTable(finalTable, p.aggregateWorker.Stat, p.aggregateWorker.RequestsThroughput)
			logger.Log.Println(buffer.String())
			return
		}
	}
}

func CreateConsoleStatusPrinter(aggregateWorker *aggregator.Aggregator) *ConsoleStatusPrinter {
	return &ConsoleStatusPrinter{
		Quit:            make(chan bool),
		aggregateWorker: aggregateWorker,
	}
}

func renderResultTable(table *tablewriter.Table, stat map[string]*common.AggregatedTestStep, requestsThroughput int) {
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

func CreateStatTable(writer io.Writer) *tablewriter.Table {
	table := tablewriter.NewWriter(writer)
	table.SetHeader([]string{"Name", "Avg. Duration", "Success", "Fail", "Total"})

	return table
}
