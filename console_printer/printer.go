package console_printer

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/gosuri/uilive"

	"postman-load-testing/aggregator"
	"postman-load-testing/logger"
)

// --- Color palette ---

var (
	cyan    = lipgloss.Color("#00D7FF")
	green   = lipgloss.Color("#00FF87")
	red     = lipgloss.Color("#FF5F87")
	yellow  = lipgloss.Color("#FFD700")
	gray    = lipgloss.Color("#6C6C6C")
	white   = lipgloss.Color("#FFFFFF")
	dimGray = lipgloss.Color("#4A4A4A")
)

// --- Styles ---

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cyan).
			PaddingLeft(1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#B0B0B0")).
			PaddingLeft(1)

	nameStyle = lipgloss.NewStyle().
			Foreground(white).
			Bold(true)

	durationStyle = lipgloss.NewStyle().
			Foreground(yellow)

	successStyle = lipgloss.NewStyle().
			Foreground(green).
			Bold(true)

	failStyle = lipgloss.NewStyle().
			Foreground(red).
			Bold(true)

	totalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#B0B0B0"))

	separatorStyle = lipgloss.NewStyle().
			Foreground(dimGray)

	metricLabelStyle = lipgloss.NewStyle().
				Foreground(gray)

	metricValueStyle = lipgloss.NewStyle().
				Foreground(cyan).
				Bold(true)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444444")).
			PaddingLeft(1).
			PaddingRight(1)

	finalBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(cyan).
			PaddingLeft(1).
			PaddingRight(1)
)

type ConsoleStatusPrinter struct {
	Quit            chan bool
	aggregateWorker *aggregator.Aggregator
	nWorkers        int
}

func (p *ConsoleStatusPrinter) Close() {
	p.Quit <- true
}

func (p *ConsoleStatusPrinter) Run() {
	timer := time.NewTicker(time.Second * 1)
	defer timer.Stop()

	writer := uilive.New()
	writer.Start()

	for {
		select {
		case <-timer.C:
			output := renderModernTable(p.aggregateWorker, p.nWorkers, false)
			_, _ = fmt.Fprint(writer, output)
			_ = writer.Flush()
		case <-p.Quit:
			writer.Stop()
			finalOutput := renderModernTable(p.aggregateWorker, p.nWorkers, true)
			fmt.Print(finalOutput)
			logger.Log.Println(finalOutput)
			return
		}
	}
}

func CreateConsoleStatusPrinter(aggregateWorker *aggregator.Aggregator, nWorkers int) *ConsoleStatusPrinter {
	return &ConsoleStatusPrinter{
		Quit:            make(chan bool),
		aggregateWorker: aggregateWorker,
		nWorkers:        nWorkers,
	}
}

func renderModernTable(agg *aggregator.Aggregator, nWorkers int, isFinal bool) string {
	stat := agg.GetStat()
	throughput := agg.GetThroughput()
	elapsed := agg.GetElapsed()
	total, totalSuccess, _ := agg.GetTotals()

	var sb strings.Builder

	// --- Header ---
	if isFinal {
		sb.WriteString(titleStyle.Render("LOAD TEST COMPLETE"))
	} else {
		dot := spinnerDot(elapsed)
		sb.WriteString(titleStyle.Render(fmt.Sprintf("%s LOAD TEST RUNNING", dot)))
	}
	sb.WriteString("\n")

	// --- Status bar ---
	elapsedStr := formatDuration(elapsed)
	statusParts := []string{
		metricLabelStyle.Render("Workers ") + metricValueStyle.Render(fmt.Sprintf("%d", nWorkers)),
		metricLabelStyle.Render("Elapsed ") + metricValueStyle.Render(elapsedStr),
		metricLabelStyle.Render("RPS ") + metricValueStyle.Render(fmt.Sprintf("%d", throughput)),
	}
	sb.WriteString(lipgloss.NewStyle().PaddingLeft(2).Render(strings.Join(statusParts, separatorStyle.Render("  |  "))))
	sb.WriteString("\n\n")

	// --- Table ---
	if len(stat) == 0 {
		sb.WriteString(lipgloss.NewStyle().PaddingLeft(2).Foreground(gray).Render("  Waiting for results..."))
		sb.WriteString("\n")
		return sb.String()
	}

	var keys []string
	for k := range stat {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Column widths
	nameW := 10
	for _, k := range keys {
		if len(stat[k].Name) > nameW {
			nameW = len(stat[k].Name)
		}
	}
	if nameW > 40 {
		nameW = 40
	}

	// Header row
	hdr := fmt.Sprintf("  %-*s  %8s  %8s  %8s  %8s  %s",
		nameW, "NAME", "AVG", "OK", "FAIL", "TOTAL", "RATE")
	sb.WriteString(headerStyle.Render(hdr))
	sb.WriteString("\n")
	sb.WriteString(separatorStyle.Render("  " + strings.Repeat("─", nameW+52)))
	sb.WriteString("\n")

	// Data rows
	for _, key := range keys {
		v := stat[key]
		name := v.Name
		if len(name) > nameW {
			name = name[:nameW-1] + "~"
		}

		avgStr := formatAvgDuration(v.AvgDuration)
		bar := makeBar(v.TotalSuccess, v.TotalFail, v.TotalCount)

		row := fmt.Sprintf("  %s  %s  %s  %s  %s  %s",
			nameStyle.Width(nameW).Render(name),
			durationStyle.Width(8).Align(lipgloss.Right).Render(avgStr),
			successStyle.Width(8).Align(lipgloss.Right).Render(fmt.Sprintf("%d", v.TotalSuccess)),
			failStyle.Width(8).Align(lipgloss.Right).Render(fmt.Sprintf("%d", v.TotalFail)),
			totalStyle.Width(8).Align(lipgloss.Right).Render(fmt.Sprintf("%d", v.TotalCount)),
			bar,
		)
		sb.WriteString(row)
		sb.WriteString("\n")
	}

	// --- Summary footer ---
	sb.WriteString(separatorStyle.Render("  " + strings.Repeat("─", nameW+52)))
	sb.WriteString("\n")

	successRate := float64(0)
	if total > 0 {
		successRate = float64(totalSuccess) / float64(total) * 100
	}

	rateColor := successStyle
	if successRate < 100 && successRate >= 95 {
		rateColor = lipgloss.NewStyle().Foreground(yellow).Bold(true)
	} else if successRate < 95 {
		rateColor = failStyle
	}

	summary := fmt.Sprintf("  %s %s    %s %s    %s %s",
		metricLabelStyle.Render("Total:"),
		metricValueStyle.Render(fmt.Sprintf("%d", total)),
		metricLabelStyle.Render("Success Rate:"),
		rateColor.Render(fmt.Sprintf("%.1f%%", successRate)),
		metricLabelStyle.Render("Throughput:"),
		metricValueStyle.Render(fmt.Sprintf("%d rps", throughput)),
	)
	sb.WriteString(summary)
	sb.WriteString("\n")

	content := sb.String()

	if isFinal {
		return "\n" + finalBoxStyle.Render(content) + "\n"
	}
	return boxStyle.Render(content) + "\n"
}

// makeBar renders a small colored bar showing success/fail ratio.
func makeBar(success, fail, total int) string {
	if total == 0 {
		return lipgloss.NewStyle().Foreground(gray).Render("--------")
	}

	barWidth := 10
	successW := barWidth * success / total
	failW := barWidth * fail / total
	if fail > 0 && failW == 0 {
		failW = 1
		if successW > 0 {
			successW--
		}
	}
	emptyW := barWidth - successW - failW

	bar := lipgloss.NewStyle().Foreground(green).Render(strings.Repeat("█", successW)) +
		lipgloss.NewStyle().Foreground(red).Render(strings.Repeat("█", failW)) +
		lipgloss.NewStyle().Foreground(dimGray).Render(strings.Repeat("░", emptyW))

	return bar
}

func formatAvgDuration(avg float64) string {
	if avg == 0 {
		return "<1ms"
	}
	if avg < 1000 {
		return fmt.Sprintf("%.0fms", avg)
	}
	return fmt.Sprintf("%.1fs", avg/1000)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}

func spinnerDot(elapsed time.Duration) string {
	frames := []string{"", "", "", ""}
	idx := int(elapsed.Seconds()) % len(frames)
	return lipgloss.NewStyle().Foreground(cyan).Render(frames[idx])
}
