package ui

import (
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/cnharrison/har-tui/internal/har"
)

type WaterfallView struct {
	*tview.List
	entries     []har.HAREntry
	indices     []int
	startTime   time.Time
	maxDuration float64
	chartWidth  int
	showDetails bool
	selectedRow int
	onSelectionChanged func(int)
}

func NewWaterfallView() *WaterfallView {
	list := tview.NewList()
	list.ShowSecondaryText(false)
	
	wv := &WaterfallView{
		List:        list,
		chartWidth:  80, // Initial value, will be updated based on terminal width
		showDetails: false,
		selectedRow: 0,
	}
	
	// Apply same colors as requests list for consistent highlighting
	wv.SetSelectedBackgroundColor(tcell.ColorDarkBlue)
	wv.SetSelectedTextColor(tcell.ColorYellow)
	wv.SetMainTextColor(tcell.ColorWhite)
	
	return wv
}

func (wv *WaterfallView) SetSelectionChangedFunc(handler func(int)) {
	wv.onSelectionChanged = handler
}

func (wv *WaterfallView) GetSelectedIndex() int {
	if wv.selectedRow < 0 || wv.selectedRow >= len(wv.indices) {
		return -1
	}
	return wv.indices[wv.selectedRow]
}

func (wv *WaterfallView) MoveUp() {
	currentItem := wv.GetCurrentItem()
	if currentItem > 0 {
		wv.SetCurrentItem(currentItem - 1)
		wv.selectedRow = currentItem - 1
		if wv.onSelectionChanged != nil {
			wv.onSelectionChanged(wv.GetSelectedIndex())
		}
	}
}

func (wv *WaterfallView) MoveDown() {
	currentItem := wv.GetCurrentItem()
	if currentItem < wv.GetItemCount()-1 {
		wv.SetCurrentItem(currentItem + 1)
		wv.selectedRow = currentItem + 1
		if wv.onSelectionChanged != nil {
			wv.onSelectionChanged(wv.GetSelectedIndex())
		}
	}
}

// scrollToSelection is no longer needed - List handles this automatically

func (wv *WaterfallView) GoToTop() {
	wv.SetCurrentItem(0)
	wv.selectedRow = 0
	if wv.onSelectionChanged != nil {
		wv.onSelectionChanged(wv.GetSelectedIndex())
	}
}

func (wv *WaterfallView) GoToBottom() {
	if wv.GetItemCount() > 0 {
		lastItem := wv.GetItemCount() - 1
		wv.SetCurrentItem(lastItem)
		wv.selectedRow = lastItem
		if wv.onSelectionChanged != nil {
			wv.onSelectionChanged(wv.GetSelectedIndex())
		}
	}
}

func (wv *WaterfallView) Update(entries []har.HAREntry, indices []int) {
	wv.entries = entries
	wv.indices = indices
	
	if len(entries) == 0 {
		wv.Clear()
		wv.AddItem("[dim]No requests to display[white]", "", 0, nil)
		return
	}
	
	// Filter out zero-duration requests for cleaner visualization
	var filteredIndices []int
	for _, idx := range indices {
		if idx >= len(entries) {
			continue
		}
		entry := entries[idx]
		if entry.Time > 0 {
			filteredIndices = append(filteredIndices, idx)
		}
	}
	
	wv.indices = filteredIndices
	
	if len(filteredIndices) == 0 {
		wv.Clear()
		wv.AddItem("[dim]No requests with measurable duration to display[white]", "", 0, nil)
		return
	}
	
	// Calculate time bounds for waterfall visualization
	wv.startTime = time.Time{}
	var endTime time.Time
	
	for _, idx := range filteredIndices {
		entry := entries[idx]
		
		startTime, err := time.Parse("2006-01-02T15:04:05.000Z", entry.StartedDateTime)
		if err != nil {
			startTime, _ = time.Parse(time.RFC3339, entry.StartedDateTime)
		}
		
		requestEndTime := startTime.Add(time.Duration(entry.Time) * time.Millisecond)
		
		if wv.startTime.IsZero() || startTime.Before(wv.startTime) {
			wv.startTime = startTime
		}
		
		if endTime.IsZero() || requestEndTime.After(endTime) {
			endTime = requestEndTime
		}
	}
	
	// maxDuration is the total timespan from first start to last end
	if !endTime.IsZero() && !wv.startTime.IsZero() {
		wv.maxDuration = endTime.Sub(wv.startTime).Seconds() * 1000
	} else {
		wv.maxDuration = 0
	}
	
	wv.renderWaterfall()
}

// SetSelectedEntry synchronizes the waterfall selection with a specific entry index
func (wv *WaterfallView) SetSelectedEntry(entryIndex int) {
	// Find the position in wv.indices that corresponds to this entry index
	for i, idx := range wv.indices {
		if idx == entryIndex {
			wv.selectedRow = i
			// Account for header items when setting list selection
			wv.SetCurrentItem(i + 2) // +2 for time scale and separator
			return
		}
	}
}

func (wv *WaterfallView) calculateTimings() {
	if len(wv.indices) == 0 {
		return
	}
	
	var earliestStart time.Time
	var latestEnd time.Time
	
	for i, idx := range wv.indices {
		if idx >= len(wv.entries) {
			continue
		}
		
		entry := wv.entries[idx]
		startTime, err := time.Parse("2006-01-02T15:04:05.000Z", entry.StartedDateTime)
		if err != nil {
			startTime, err = time.Parse(time.RFC3339, entry.StartedDateTime)
			if err != nil {
				continue
			}
		}
		
		endTime := startTime.Add(time.Duration(entry.Time) * time.Millisecond)
		
		if i == 0 || startTime.Before(earliestStart) {
			earliestStart = startTime
		}
		if i == 0 || endTime.After(latestEnd) {
			latestEnd = endTime
		}
	}
	
	wv.startTime = earliestStart
	wv.maxDuration = latestEnd.Sub(earliestStart).Seconds() * 1000
}

func (wv *WaterfallView) renderWaterfall() {
	if len(wv.indices) == 0 {
		return
	}
	
	// Update chart width based on current terminal width
	_, _, viewWidth, _ := wv.GetInnerRect()
	if viewWidth > 0 {
		// Reserve space for request info columns (method + status + host + path + separator ≈ 70 chars)
		availableWidth := viewWidth - 70
		if availableWidth > 20 { // Remove upper bound to use full terminal width
			wv.chartWidth = availableWidth
		}
	}
	
	// Clear the list and rebuild it
	wv.Clear()
	
	// Add header items
	wv.AddItem(wv.renderTimeScale(), "", 0, nil)
	wv.AddItem(strings.Repeat("─", viewWidth), "", 0, nil)
	
	sortedIndices := make([]int, len(wv.indices))
	copy(sortedIndices, wv.indices)
	
	sort.Slice(sortedIndices, func(i, j int) bool {
		entryI := wv.entries[sortedIndices[i]]
		entryJ := wv.entries[sortedIndices[j]]
		
		startTimeI, _ := time.Parse("2006-01-02T15:04:05.000Z", entryI.StartedDateTime)
		startTimeJ, _ := time.Parse("2006-01-02T15:04:05.000Z", entryJ.StartedDateTime)
		
		return startTimeI.Before(startTimeJ)
	})
	
	// Add each request as a list item
	for i, idx := range sortedIndices {
		if idx >= len(wv.entries) {
			continue
		}
		entry := wv.entries[idx]
		
		requestBar := wv.renderRequestBar(entry, i, false) // Lists handle selection automatically
		wv.AddItem(requestBar, "", 0, nil)
	}
	
	// Restore selection
	if wv.selectedRow >= 0 && wv.selectedRow < wv.GetItemCount()-2 { // Account for header items
		wv.SetCurrentItem(wv.selectedRow + 2) // +2 for header items
	}
}

func (wv *WaterfallView) renderTimeScale() string {
	var scale strings.Builder
	
	// Fixed-width header to align with request info columns
	headerWidth := 4 + 1 + 3 + 1 + 25 + 1 + 35 + 1 + 1 // method + status + host + path + separator
	scale.WriteString(fmt.Sprintf("%-*s", headerWidth, "[white]Time Scale (log):"))
	
	// Calculate tick marks for the logarithmic time scale
	for i := 0; i <= 10; i++ {
		progress := float64(i) / 10.0
		// Convert back from log scale to actual time
		logMax := math.Log10(wv.maxDuration + 1)
		actualTime := math.Pow(10, progress*logMax) - 1
		
		tickPos := int(float64(wv.chartWidth) * progress)
		
		// Add spacing to position the tick mark
		if i > 0 {
			prevProgress := float64(i-1) / 10.0
			prevTickPos := int(float64(wv.chartWidth) * prevProgress)
			spacing := tickPos - prevTickPos - 4 // Account for previous label width
			if spacing > 0 {
				scale.WriteString(strings.Repeat(" ", spacing))
			}
		}
		
		if actualTime < 1000 {
			scale.WriteString(fmt.Sprintf("[dim]%.0f[white]", actualTime))
		} else {
			scale.WriteString(fmt.Sprintf("[dim]%.1fs[white]", actualTime/1000))
		}
	}
	
	return scale.String()
}

func (wv *WaterfallView) renderRequestBar(entry har.HAREntry, rowIndex int, isSelected bool) string {
	startTime, err := time.Parse("2006-01-02T15:04:05.000Z", entry.StartedDateTime)
	if err != nil {
		startTime, _ = time.Parse(time.RFC3339, entry.StartedDateTime)
	}
	
	relativeStart := startTime.Sub(wv.startTime).Seconds() * 1000
	duration := entry.Time
	
	// Calculate relative positioning within the filtered timespan
	var startPos, barWidth int
	
	// Debug: Let's see what values we're working with (remove this later)
	// For now, let's use a simpler approach: if maxDuration is very large, 
	// the bars will all be positioned at 0. Let's fix this.
	
	if wv.maxDuration <= 100 { // If timespan is very small (< 100ms), don't use positioning
		// All requests at nearly same time - use duration-only scaling without positioning
		startPos = 0
	} else if wv.maxDuration > 10000 { // If timespan > 10 seconds, use simplified positioning
		// Large timespan - use request order for staggered positioning instead of time-based
		// This gives a waterfall effect even when actual times are too close
		startPos = rowIndex * 2 // Stagger by 2 characters per request
		if startPos > wv.chartWidth/3 { // Don't go beyond 1/3 of chart width
			startPos = (rowIndex % (wv.chartWidth/6)) * 2
		}
	} else {
		// Medium timespan - use time-based positioning
		startPos = int(float64(wv.chartWidth) * relativeStart / wv.maxDuration)
	}
	
	// Width calculation: hybrid scaling for better visibility
	if wv.maxDuration <= 0 {
		// No timespan - scale by individual durations only
		maxDurationInSet := 0.0
		for _, idx := range wv.indices {
			if idx < len(wv.entries) && wv.entries[idx].Time > maxDurationInSet {
				maxDurationInSet = wv.entries[idx].Time
			}
		}
		if maxDurationInSet > 0 && duration > 0 {
			rawBarWidth := float64(wv.chartWidth) * duration / maxDurationInSet
			barWidth = int(rawBarWidth)
			if barWidth < 1 {
				barWidth = 1
			}
		} else {
			barWidth = 1
		}
	} else {
		// Hybrid approach: use duration-based scaling when timespan scaling is too small
		timespanBasedWidth := float64(wv.chartWidth) * duration / wv.maxDuration
		
		if timespanBasedWidth < 5.0 && duration > 20 { // If request > 20ms but bar < 5 pixels
			// Find max duration in current filtered set for proportional scaling
			maxDurationInSet := 0.0
			for _, idx := range wv.indices {
				if idx < len(wv.entries) && wv.entries[idx].Time > maxDurationInSet {
					maxDurationInSet = wv.entries[idx].Time
				}
			}
			
			if maxDurationInSet > 0 {
				// Scale based on durations with more available width
				durationBasedWidth := float64(wv.chartWidth) * 0.8 * duration / maxDurationInSet
				barWidth = int(durationBasedWidth)
				if barWidth < 3 && duration > 100 {
					barWidth = 3 // Minimum 3 pixels for requests > 100ms
				} else if barWidth < 2 && duration > 20 {
					barWidth = 2 // Minimum 2 pixels for requests > 20ms
				} else if barWidth < 1 && duration > 0 {
					barWidth = 1 // Minimum 1 pixel for any request
				}
			} else {
				barWidth = int(timespanBasedWidth)
				if barWidth < 1 && duration > 0 {
					barWidth = 1
				}
			}
		} else {
			// Use timespan-based width when it provides adequate visibility
			barWidth = int(timespanBasedWidth)
			if barWidth < 1 && duration > 0 {
				barWidth = 1
			}
		}
	}
	
	u, _ := url.Parse(entry.Request.URL)
	host := u.Host
	path := u.Path
	
	method := entry.Request.Method
	status := entry.Response.Status
	
	statusColor := "white"
	if status >= 400 {
		statusColor = "red"
	} else if status >= 300 {
		statusColor = "yellow"
	} else if status >= 200 {
		statusColor = "green"
	}
	
	var bar strings.Builder
	
	// Fixed-width columns with proper selection highlighting (using inverse colors, not backgrounds)
	var methodCol, statusCol, hostCol, pathCol string
	if isSelected {
		// Use bright colors instead of background colors to avoid white background issues
		methodCol = fmt.Sprintf("[yellow:black]%-4s[-]", method)
		statusCol = fmt.Sprintf("[yellow:black]%3d[-]", status)
		hostCol = fmt.Sprintf("[yellow:black]%-25s[-]", truncateString(host, 25))
		pathCol = fmt.Sprintf("[yellow:black]%-35s[-]", truncateString(path, 35))
	} else {
		methodCol = fmt.Sprintf("[cyan]%-4s[-]", method)
		statusCol = fmt.Sprintf("[%s]%3d[-]", statusColor, status)
		hostCol = fmt.Sprintf("[blue]%-25s[-]", truncateString(host, 25))
		pathCol = fmt.Sprintf("[dim]%-35s[-]", truncateString(path, 35))
	}
	
	bar.WriteString(fmt.Sprintf("%s %s %s %s │", methodCol, statusCol, hostCol, pathCol))
	
	// Add spacing to align bars
	bar.WriteString(strings.Repeat(" ", startPos))
	
	// Timing bars are never highlighted - keep original colors
	barColor := wv.getBarColor(entry)
	if wv.showDetails {
		bar.WriteString(wv.renderDetailedBar(entry, barWidth))
	} else {
		bar.WriteString(fmt.Sprintf("[%s]%s[-]", barColor, strings.Repeat("█", barWidth)))
	}
	
	// Duration text with selection highlighting
	if isSelected {
		bar.WriteString(fmt.Sprintf(" [yellow:black]%.1fms[-]", duration))
	} else {
		bar.WriteString(fmt.Sprintf(" [yellow]%.1fms[-]", duration))
	}
	
	return bar.String()
}

func (wv *WaterfallView) renderDetailedBar(entry har.HAREntry, totalWidth int) string {
	timings := entry.Timings
	totalTime := entry.Time
	
	phases := []struct {
		name     string
		duration float64
		color    string
		char     string
	}{
		{"blocked", timings.Blocked, "red", "▓"},
		{"dns", timings.DNS, "blue", "▓"},
		{"connect", timings.Connect, "green", "▓"},
		{"ssl", timings.SSL, "magenta", "▓"},
		{"send", timings.Send, "cyan", "▓"},
		{"wait", timings.Wait, "yellow", "▓"},
		{"receive", timings.Receive, "white", "▓"},
	}
	
	var bar strings.Builder
	usedWidth := 0
	
	for _, phase := range phases {
		if phase.duration > 0 {
			phaseWidth := int(float64(totalWidth) * phase.duration / totalTime)
			if phaseWidth < 1 && phase.duration > 0 {
				phaseWidth = 1
			}
			
			if usedWidth+phaseWidth > totalWidth {
				phaseWidth = totalWidth - usedWidth
			}
			
			if phaseWidth > 0 {
				bar.WriteString(fmt.Sprintf("[%s]%s[-]", phase.color, strings.Repeat(phase.char, phaseWidth)))
				usedWidth += phaseWidth
			}
		}
	}
	
	if usedWidth < totalWidth {
		bar.WriteString(strings.Repeat("░", totalWidth-usedWidth))
	}
	
	return bar.String()
}

func (wv *WaterfallView) getBarColor(entry har.HAREntry) string {
	requestType := har.GetRequestType(entry)
	
	switch requestType {
	case "doc":
		return "blue"
	case "css":
		return "green"
	case "js":
		return "yellow"
	case "img":
		return "magenta"
	case "fetch":
		return "cyan"
	case "media":
		return "red"
	default:
		return "white"
	}
}

func (wv *WaterfallView) ToggleDetails() {
	wv.showDetails = !wv.showDetails
	wv.renderWaterfall()
}

func (wv *WaterfallView) SetChartWidth(width int) {
	if width > 20 && width < 200 {
		wv.chartWidth = width
		wv.renderWaterfall()
	}
}

func (wv *WaterfallView) ZoomIn() {
	if wv.chartWidth < 150 {
		wv.chartWidth += 10
		wv.renderWaterfall()
	}
}

func (wv *WaterfallView) ZoomOut() {
	if wv.chartWidth > 30 {
		wv.chartWidth -= 10
		wv.renderWaterfall()
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
