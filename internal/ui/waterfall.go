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

const (
	// Layout constants
	defaultChartWidth = 80
	requestInfoColumnsWidth = 70  // method + status + host + path + separator
	minChartWidth = 20
	stickyHeaderHeight = 2
	
	// Timing thresholds
	smallTimespanMs = 100    // < 100ms considered small timespan
	largeTimespanMs = 10000  // > 10s considered large timespan
	staggerOffset = 2        // pixels between staggered bars
	
	// Bar width thresholds  
	minBarWidthThreshold = 5.0   // pixels - switch to duration scaling
	minDurationForScaling = 20   // ms - minimum duration for special scaling
	minBarWidthLarge = 3         // pixels for requests > 100ms
	minBarWidthSmall = 2         // pixels for requests > 20ms
	durationScalingFactor = 0.8  // use 80% of chart width for duration scaling
)


type WaterfallView struct {
	*tview.Flex
	headerView  *tview.TextView
	listView    *tview.List
	entries     []har.HAREntry
	indices     []int
	startTime   time.Time
	maxDuration float64
	chartWidth  int
	showDetails bool
	manuallyResized bool  // Flag to prevent auto-sizing from overriding manual zoom
	onSelectionChanged func(int)
}

func NewWaterfallView() *WaterfallView {
	// Create header for time scale (sticky)
	headerView := tview.NewTextView()
	headerView.SetDynamicColors(true)
	headerView.SetWrap(false)
	
	// Create list for requests (scrollable)
	listView := tview.NewList()
	listView.ShowSecondaryText(false)
	listView.SetSelectedBackgroundColor(tcell.ColorDarkBlue)
	listView.SetSelectedTextColor(tcell.ColorYellow)
	listView.SetMainTextColor(tcell.ColorWhite)
	
	// Create flex container
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(headerView, stickyHeaderHeight, 0, false)  // Fixed height for header (time scale + separator)
	flex.AddItem(listView, 0, 1, true)     // Flexible height for scrollable requests
	
	wv := &WaterfallView{
		Flex:        flex,
		headerView:  headerView,
		listView:    listView,
		chartWidth:  defaultChartWidth, // Initial value, will be updated based on terminal width
		showDetails: false,
		manuallyResized: false,  // Start with auto-sizing enabled
	}
	
	// Set up native selection change callback for j/k key navigation
	listView.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		if wv.onSelectionChanged != nil {
			wv.onSelectionChanged(wv.GetSelectedIndex())
		}
	})
	
	return wv
}

func (wv *WaterfallView) SetSelectionChangedFunc(handler func(int)) {
	wv.onSelectionChanged = handler
}

func (wv *WaterfallView) GetSelectedIndex() int {
	currentRow := wv.listView.GetCurrentItem()
	if currentRow < 0 || currentRow >= len(wv.indices) {
		return -1
	}
	return wv.indices[currentRow]
}

func (wv *WaterfallView) MoveUp() {
	currentItem := wv.listView.GetCurrentItem()
	if currentItem > 0 {
		wv.listView.SetCurrentItem(currentItem - 1)
		// SetCurrentItem will trigger SetChangedFunc which handles the callback
	}
}

func (wv *WaterfallView) MoveDown() {
	currentItem := wv.listView.GetCurrentItem()
	if currentItem < wv.listView.GetItemCount()-1 {
		wv.listView.SetCurrentItem(currentItem + 1)
		// SetCurrentItem will trigger SetChangedFunc which handles the callback
	}
}

// scrollToSelection is no longer needed - List handles this automatically

func (wv *WaterfallView) GoToTop() {
	wv.listView.SetCurrentItem(0)
	// SetCurrentItem will trigger SetChangedFunc which handles the callback
}

func (wv *WaterfallView) GoToBottom() {
	if wv.listView.GetItemCount() > 0 {
		lastItem := wv.listView.GetItemCount() - 1
		wv.listView.SetCurrentItem(lastItem)
		// SetCurrentItem will trigger SetChangedFunc which handles the callback
	}
}

func (wv *WaterfallView) Update(entries []har.HAREntry, indices []int) {
	wv.entries = entries
	wv.indices = indices
	
	if len(entries) == 0 {
		wv.listView.Clear()
		wv.listView.AddItem("[dim]No requests to display[white]", "", 0, nil)
		wv.headerView.SetText("")
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
		wv.listView.Clear()
		wv.listView.AddItem("[dim]No requests with measurable duration to display[white]", "", 0, nil)
		wv.headerView.SetText("")
		return
	}
	
	// Calculate time bounds for waterfall visualization
	wv.startTime = time.Time{}
	var endTime time.Time
	
	for _, idx := range filteredIndices {
		entry := entries[idx]
		
		startTime, err := har.ParseHARDateTime(entry.StartedDateTime)
		if err != nil {
			continue // Skip entries with unparseable timestamps
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
			// SetCurrentItem will trigger SetChangedFunc to update selectedRow
			wv.listView.SetCurrentItem(i)
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
		startTime, err := har.ParseHARDateTime(entry.StartedDateTime)
		if err != nil {
			continue // Skip entries with unparseable timestamps
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
	
	// Update chart width based on current terminal width (only if not manually resized)
	_, _, viewWidth, _ := wv.GetInnerRect()
	if viewWidth > 0 && !wv.manuallyResized {
		// Reserve space for request info columns
		availableWidth := viewWidth - requestInfoColumnsWidth
		if availableWidth > minChartWidth { // Remove upper bound to use full terminal width
			wv.chartWidth = availableWidth
		}
	}
	
	// Update sticky header
	headerText := wv.renderTimeScale() + "\n" + strings.Repeat("─", viewWidth)
	wv.headerView.SetText(headerText)
	
	// Clear the list and rebuild content (no header items)
	wv.listView.Clear()
	
	sortedIndices := make([]int, len(wv.indices))
	copy(sortedIndices, wv.indices)
	
	sort.Slice(sortedIndices, func(i, j int) bool {
		entryI := wv.entries[sortedIndices[i]]
		entryJ := wv.entries[sortedIndices[j]]
		
		startTimeI, errI := har.ParseHARDateTime(entryI.StartedDateTime)
		startTimeJ, errJ := har.ParseHARDateTime(entryJ.StartedDateTime)
		
		// Handle parsing errors - entries with unparseable times go last
		if errI != nil && errJ == nil {
			return false // I goes after J
		}
		if errI == nil && errJ != nil {
			return true // I goes before J
		}
		if errI != nil && errJ != nil {
			return false // Both unparseable, maintain original order
		}
		
		return startTimeI.Before(startTimeJ)
	})
	
	// Add each request as a list item
	for i, idx := range sortedIndices {
		if idx >= len(wv.entries) {
			continue
		}
		entry := wv.entries[idx]
		
		requestBar := wv.renderRequestBar(entry, i, false) // Lists handle selection automatically
		wv.listView.AddItem(requestBar, "", 0, nil)
	}
	
	// Preserve current selection during re-render
	currentSelection := wv.listView.GetCurrentItem()
	if currentSelection >= 0 && currentSelection < wv.listView.GetItemCount() {
		wv.listView.SetCurrentItem(currentSelection)
	}
}

func (wv *WaterfallView) renderTimeScale() string {
	var scale strings.Builder
	
	// Fixed-width header to align with request info columns
	headerWidth := requestInfoColumnsWidth // method + status + host + path + separator
	scale.WriteString(fmt.Sprintf("%-*s", headerWidth, "[white]Time Scale (log):"))
	
	// Calculate tick marks for the logarithmic time scale
	var labels []string
	var positions []int
	
	for i := 0; i <= 10; i++ {
		progress := float64(i) / 10.0
		// Convert back from log scale to actual time
		logMax := math.Log10(wv.maxDuration + 1)
		actualTime := math.Pow(10, progress*logMax) - 1
		
		tickPos := int(float64(wv.chartWidth) * progress)
		
		// Format label consistently
		var label string
		if actualTime < 1000 {
			label = fmt.Sprintf("[dim]%.0f[white]", actualTime)
		} else {
			label = fmt.Sprintf("[dim]%.1fs[white]", actualTime/1000)
		}
		
		labels = append(labels, label)
		positions = append(positions, tickPos)
	}
	
	// Render with proper spacing based on actual label lengths
	for i, label := range labels {
		if i > 0 {
			// Calculate spacing based on previous label's actual length and position
			prevLabelLen := len(stripColorTags(labels[i-1]))
			spacing := positions[i] - positions[i-1] - prevLabelLen
			if spacing > 0 {
				scale.WriteString(strings.Repeat(" ", spacing))
			}
		}
		scale.WriteString(label)
	}
	
	return scale.String()
}

// stripColorTags removes tview color tags to get actual display length
func stripColorTags(text string) string {
	// Simple regex replacement would be better, but this works for our case
	result := strings.ReplaceAll(text, "[dim]", "")
	result = strings.ReplaceAll(result, "[white]", "")
	return result
}

func (wv *WaterfallView) renderRequestBar(entry har.HAREntry, rowIndex int, isSelected bool) string {
	startTime, err := har.ParseHARDateTime(entry.StartedDateTime)
	if err != nil {
		// Use current time as fallback for rendering purposes
		startTime = time.Now()
	}
	
	relativeStart := startTime.Sub(wv.startTime).Seconds() * 1000
	duration := entry.Time
	
	// Calculate relative positioning within the filtered timespan
	var startPos, barWidth int
	
	// Debug: Let's see what values we're working with (remove this later)
	// For now, let's use a simpler approach: if maxDuration is very large, 
	// the bars will all be positioned at 0. Let's fix this.
	
	if wv.maxDuration <= smallTimespanMs { // If timespan is very small, don't use positioning
		// All requests at nearly same time - use duration-only scaling without positioning
		startPos = 0
	} else if wv.maxDuration > largeTimespanMs { // If timespan > 10 seconds, use simplified positioning
		// Large timespan - use request order for staggered positioning instead of time-based
		// This gives a waterfall effect even when actual times are too close
		startPos = rowIndex * staggerOffset // Stagger by offset pixels per request
		if startPos > wv.chartWidth/3 { // Don't go beyond 1/3 of chart width
			startPos = (rowIndex % (wv.chartWidth/6)) * staggerOffset
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
		
		if timespanBasedWidth < minBarWidthThreshold && duration > minDurationForScaling { // If request > 20ms but bar < 5 pixels
			// Find max duration in current filtered set for proportional scaling
			maxDurationInSet := 0.0
			for _, idx := range wv.indices {
				if idx < len(wv.entries) && wv.entries[idx].Time > maxDurationInSet {
					maxDurationInSet = wv.entries[idx].Time
				}
			}
			
			if maxDurationInSet > 0 {
				// Scale based on durations with more available width
				durationBasedWidth := float64(wv.chartWidth) * durationScalingFactor * duration / maxDurationInSet
				barWidth = int(durationBasedWidth)
				if barWidth < minBarWidthLarge && duration > smallTimespanMs {
					barWidth = minBarWidthLarge // Minimum 3 pixels for requests > 100ms
				} else if barWidth < minBarWidthSmall && duration > minDurationForScaling {
					barWidth = minBarWidthSmall // Minimum 2 pixels for requests > 20ms
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
	if status == 0 {
		statusColor = "red"  // Aborted/cancelled requests
	} else if status >= 400 {
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
	
	// Add CORS indicator for CORS errors
	corsIndicator := ""
	if har.IsCORSError(entry) {
		if isSelected {
			corsIndicator = " [yellow:black]CORS[-]"
		} else {
			corsIndicator = " [red]CORS[-]"
		}
	}
	
	bar.WriteString(fmt.Sprintf("%s %s %s %s%s │", methodCol, statusCol, hostCol, pathCol, corsIndicator))
	
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
		wv.manuallyResized = true  // Mark as manually resized to prevent auto-sizing
		wv.renderWaterfall()
	}
}

// ResetZoom resets to auto-sizing behavior based on terminal width
func (wv *WaterfallView) ResetZoom() {
	wv.manuallyResized = false
	wv.renderWaterfall()  // This will trigger auto-sizing since manuallyResized is now false
}

func (wv *WaterfallView) ZoomIn() {
	if wv.chartWidth < 150 {
		wv.chartWidth += 10
		wv.manuallyResized = true  // Mark as manually resized to prevent auto-sizing
		wv.renderWaterfall()
	}
}

func (wv *WaterfallView) ZoomOut() {
	if wv.chartWidth > 30 {
		wv.chartWidth -= 10
		wv.manuallyResized = true  // Mark as manually resized to prevent auto-sizing
		wv.renderWaterfall()
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
