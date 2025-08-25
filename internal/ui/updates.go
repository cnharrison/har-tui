package ui

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tidwall/gjson"
	"github.com/cnharrison/har-tui/internal/filter"
	"github.com/cnharrison/har-tui/internal/har"
)

func (app *Application) updateRequestsList() {
	app.requests.Clear()

	var entries []har.HAREntry
	if app.isLoading {
		entries = app.streamingLoader.GetEntries()
		if len(entries) > 0 {
			app.filteredEntries = app.filterState.FilterEntriesWithIndex(entries, app.streamingLoader.GetIndex())
		} else {
			app.filteredEntries = []int{}
		}
	} else if app.harData != nil {
		entries = app.harData.Log.Entries
		app.filteredEntries = app.filterState.FilterEntries(entries)
	} else {
		return
	}

	for _, idx := range app.filteredEntries {
		if idx >= len(entries) {
			continue
		}
		entry := entries[idx]
		
		// Create display text for the request
		u, _ := url.Parse(entry.Request.URL)
		host := u.Host
		path := u.Path
		if len(path) > maxPathDisplayLength {
			path = path[:maxPathDisplayLength-pathTruncateOffset] + "..."
		}
		
		method := entry.Request.Method
		status := entry.Response.Status
		duration := fmt.Sprintf("%.0fms", entry.Time)
		
		// Color code by status
		statusColor := "white"
		if status == 0 {
			statusColor = "red"  // Aborted/cancelled requests
		} else if status >= statusCodeClientError {
			statusColor = "red"
		} else if status >= statusCodeRedirect {
			statusColor = "yellow"
		} else if status >= statusCodeSuccess {
			statusColor = "green"
		}
		
		// Add CORS indicator for CORS errors
		corsIndicator := ""
		if har.IsCORSError(entry) {
			corsIndicator = " [red]CORS[white]"
		}
		
		displayText := fmt.Sprintf("[cyan]%-4s[white] [%s]%3d[white] [blue]%s[white] [dim]%s[white] [yellow]%s[white]%s", 
			method, statusColor, status, host, path, duration, corsIndicator)
		
		app.requests.AddItem(displayText, "", 0, nil)
	}
	
	// Update waterfall view if it's currently shown
	if app.showWaterfall {
		app.updateWaterfallView()
	}
	
	// Update selection if we have items
	if len(app.filteredEntries) > 0 {
		currentItem := app.requests.GetCurrentItem()
		if currentItem >= len(app.filteredEntries) {
			app.requests.SetCurrentItem(0)
			// Force update since SetCurrentItem might not always trigger SetChangedFunc
			app.updateTabContent(0)
		} else {
			// Always refresh tab content to ensure it matches the current selection
			app.updateTabContent(currentItem)
		}
	} else {
		// Clear content when no entries are available
		app.bodyView.SetText("[dim]No requests match the current filter[white]")
	}
}

// updateTabContent updates the content of the currently selected tab
func (app *Application) updateTabContent(selectedIndex int) {
	if selectedIndex < 0 || selectedIndex >= len(app.filteredEntries) {
		return
	}
	
	entryIdx := app.filteredEntries[selectedIndex]
	
	var entries []har.HAREntry
	if app.isLoading {
		entries = app.streamingLoader.GetEntries()
	} else if app.harData != nil {
		entries = app.harData.Log.Entries
	} else {
		return
	}
	
	if entryIdx >= len(entries) {
		return
	}
	
	entry := entries[entryIdx]
	
	// Request tab
	reqHeaders := app.formatter.FormatContent(app.prettyJSON(entry.Request.Headers), "json")
	reqPostData := "[dim]None[white]"
	if entry.Request.PostData != nil {
		contentType := app.formatter.DetectContentType(entry.Request.PostData.Text, entry.Request.PostData.MimeType)
		reqPostData = app.formatter.FormatContent(entry.Request.PostData.Text, contentType)
	}
	
	app.requestView.SetText(fmt.Sprintf(
		"[yellow]Method:[white] [cyan]%s[white]\n[yellow]URL:[white] [blue]%s[white]\n[yellow]HTTP Version:[white] %s\n\n[yellow]Headers:[white]\n%s\n\n[yellow]Post Data:[white]\n%s",
		entry.Request.Method,
		entry.Request.URL,
		entry.Request.HTTPVersion,
		reqHeaders,
		reqPostData,
	))
	
	// Response tab
	respHeaders := app.formatter.FormatContent(app.prettyJSON(entry.Response.Headers), "json")
	statusColor := "white"
	if entry.Response.Status == 0 {
		statusColor = "red"  // Aborted/cancelled requests
	} else if entry.Response.Status >= statusCodeClientError {
		statusColor = "red"
	} else if entry.Response.Status >= statusCodeRedirect {
		statusColor = "yellow"
	} else if entry.Response.Status >= statusCodeSuccess {
		statusColor = "green"
	}
	
	app.responseView.SetText(fmt.Sprintf(
		"[yellow]Status:[white] [%s]%d %s[white]\n[yellow]HTTP Version:[white] %s\n[yellow]Content Type:[white] [cyan]%s[white]\n[yellow]Size:[white] [yellow]%d[white] bytes\n\n[yellow]Headers:[white]\n%s",
		statusColor,
		entry.Response.Status,
		entry.Response.StatusText,
		entry.Response.HTTPVersion,
		entry.Response.Content.MimeType,
		entry.Response.Content.Size,
		respHeaders,
	))
	
	// Body tab with intelligent formatting
	bodyText := har.DecodeBase64(entry.Response.Content.Text, entry.Response.Content.Encoding)
	if bodyText != "" {
		contentType := app.formatter.DetectContentType(bodyText, entry.Response.Content.MimeType)
		formattedBodyText := app.formatter.FormatContent(bodyText, contentType)
		
		// Check if this needs tview native layout (side-by-side)
		if strings.HasPrefix(formattedBodyText, "TVIEW_LAYOUT:") {
			app.setupSideBySideBodyView(formattedBodyText)
		} else {
			// Ensure we're using the normal body view (restore if we had side-by-side)
			app.restoreNormalBodyView()
			
			// Use JSON line highlighting if it's JSON content
			if contentType == "json" {
				// Pretty-print the JSON first so we have multiple lines to highlight
				prettyJSON := app.prettyPrintJSON(bodyText)
				// Apply syntax highlighting first
				syntaxHighlighted := app.formatter.FormatContent(prettyJSON, contentType)
				// Then apply line highlighting to the syntax-highlighted content
				finalText := app.formatJSONWithHighlight(syntaxHighlighted, entryIdx)
				app.bodyView.SetText(finalText)
			} else {
				app.bodyView.SetText(formattedBodyText)
			}
		}
	} else {
		// Ensure we're using the normal body view for empty content too
		app.restoreNormalBodyView()
		app.bodyView.SetText("[dim]No body content[white]")
	}
	
	// Cookies tab
	cookieText := "[yellow]Request Cookies:[white]\n"
	if len(entry.Request.Cookies) > 0 {
		cookieText += app.prettyJSON(entry.Request.Cookies)
	} else {
		cookieText += "[dim]None[white]"
	}
	cookieText += "\n\n[yellow]Response Cookies:[white]\n"
	if len(entry.Response.Cookies) > 0 {
		cookieText += app.prettyJSON(entry.Response.Cookies)
	} else {
		cookieText += "[dim]None[white]"
	}
	app.cookiesView.SetText(cookieText)
	
	// Timings tab
	app.timingsView.SetText(app.formatTimings(entry.Timings, entry.Time))
	
	
	// Raw tab
	app.rawView.SetText(fmt.Sprintf("[yellow]Complete Entry:[white]\n\n%s", app.prettyJSON(entry)))
	
	// Update bottom bar to reflect new context
	app.updateBottomBar()
}

// updateFilterBar updates the filter button bar
func (app *Application) updateFilterBar() {
	typeFilters := filter.GetTypeFilters()
	var filterText strings.Builder
	
	for i, filterType := range typeFilters {
		if i == app.selectedFilterIndex && filterType == app.filterState.ActiveTypeFilter {
			// Selected and active - bright highlight
			filterText.WriteString(fmt.Sprintf("[black:yellow:b] %s [white:black:-] ", strings.ToUpper(filterType)))
		} else if i == app.selectedFilterIndex {
			// Just selected (navigating) - dim highlight
			filterText.WriteString(fmt.Sprintf("[magenta:blue:b] %s [white:black:-] ", strings.ToUpper(filterType)))
		} else if filterType == app.filterState.ActiveTypeFilter {
			// Just active - colored but not selected
			filterText.WriteString(fmt.Sprintf("[magenta:green:b] %s [white:black:-] ", strings.ToUpper(filterType)))
		} else {
			// Regular filter button - boxed appearance
			filterText.WriteString(fmt.Sprintf("[magenta:black:b] %s [white:black:-] ", strings.ToUpper(filterType)))
		}
	}
	
	app.filterBar.SetText(filterText.String())
}

// updateTabBar updates the tab indicator bar
func (app *Application) updateTabBar() {
	tabNames := []string{"Request", "Response", "Body", "Cookies", "Timings", "Raw"}
	var tabText strings.Builder
	
	for i, name := range tabNames {
		if i == app.currentTab {
			tabText.WriteString(fmt.Sprintf("[black:white] %s [white:black]", name))
		} else {
			tabText.WriteString(fmt.Sprintf(" [blue]%s[white] ", name))
		}
		if i < len(tabNames)-1 {
			tabText.WriteString(" │ ")
		}
	}
	
	app.tabBar.SetText(tabText.String())
}

// updateWaterfallView updates the waterfall view with current filtered entries
func (app *Application) updateWaterfallView() {
	var waterfallEntries []har.HAREntry
	if app.isLoading {
		waterfallEntries = app.streamingLoader.GetEntries()
	} else if app.harData != nil {
		waterfallEntries = app.harData.Log.Entries
	}
	if len(waterfallEntries) > 0 {
		app.waterfallView.Update(waterfallEntries, app.filteredEntries)
		
		// Synchronize waterfall selection with requests list selection
		currentRequestsIndex := app.requests.GetCurrentItem()
		if currentRequestsIndex >= 0 && currentRequestsIndex < len(app.filteredEntries) {
			selectedEntryIndex := app.filteredEntries[currentRequestsIndex]
			app.waterfallView.SetSelectedEntry(selectedEntryIndex)
		}
	}
}

// updateBottomBar updates the status/bottom bar
func (app *Application) updateBottomBar() {
	var statusText strings.Builder
	
	// Check if we have an active confirmation message
	if time.Now().Before(app.confirmationEnd) && app.confirmationMessage != "" {
		// Show animated confirmation
		pulse := []string{"●", "◐", "◑", "◒", "◓", "○"}
		pulseFrame := (app.animationFrame / pulseCycleFrames) % len(pulse)
		
		statusText.WriteString(fmt.Sprintf(" [yellow]%s [white]%s", 
			pulse[pulseFrame], app.confirmationMessage))
	} else {
		// Show regular status
		app.confirmationMessage = ""
		
		var totalEntries int
		if app.isLoading {
			totalEntries = app.streamingLoader.GetEntryCount()
			if app.loadingProgress > 0 {
				statusText.WriteString(fmt.Sprintf("Loading... %d entries", app.loadingProgress))
			} else {
				statusText.WriteString("Starting load...")
			}
			if len(app.filteredEntries) > 0 {
				statusText.WriteString(fmt.Sprintf(" | Showing %d/%d", len(app.filteredEntries), totalEntries))
			}
		} else if app.harData != nil {
			totalEntries = len(app.harData.Log.Entries)
			statusText.WriteString(fmt.Sprintf("Showing %d/%d requests", len(app.filteredEntries), totalEntries))
		}
		
		if app.filterState.FilterText != "" {
			statusText.WriteString(fmt.Sprintf(" | Filter: [cyan]%s[white]", app.filterState.FilterText))
		}
		if app.filterState.ShowErrorsOnly {
			statusText.WriteString(" | [red]Errors Only[white]")
		}
		if app.filterState.SortBySlowest {
			statusText.WriteString(" | [yellow]Sorted by Time[white]")
		}
		if app.filterState.ActiveTypeFilter != "all" {
			statusText.WriteString(fmt.Sprintf(" | [cyan]Type: %s[white]", app.filterState.ActiveTypeFilter))
		}
	}
	
	// Add contextual information on the right side
	contextInfo := app.getContentContext()
	// Force display of context info even if it's empty for debugging
	if contextInfo != "" || true {
		if contextInfo == "" {
			contextInfo = "[dim]DEBUG: no context[white]"
		}
		// Calculate padding to right-align context info
		_, _, width, _ := app.bottomBar.GetRect()
		leftText := " " + statusText.String() + " "
		rightText := " " + contextInfo + " "
		
		// Calculate visible lengths (without color codes)
		leftLen := calculateVisibleLength(leftText)
		rightLen := calculateVisibleLength(rightText)
		
		// Always show context info, even if width calculation fails or space is tight
		if width > 0 && leftLen + rightLen < width {
			padding := width - leftLen - rightLen
			finalText := leftText + strings.Repeat(" ", padding) + rightText
			app.bottomBar.SetText(finalText)
		} else {
			// Fallback: intelligently truncate rightText to fit if needed
			maxRightWidth := width - leftLen - 2 // Leave some margin
			if maxRightWidth > 10 && rightLen > maxRightWidth {
				// Truncate context info intelligently, preserving important parts
				truncatedRight := app.truncateContextInfo(rightText, maxRightWidth)
				app.bottomBar.SetText(leftText + truncatedRight)
			} else {
				// Show both left and right with minimal spacing
				app.bottomBar.SetText(leftText + rightText)
			}
		}
	} else {
		app.bottomBar.SetText(" " + statusText.String() + " ")
	}
}

// updateBottomBarSafe updates the status bar without interfering with JSON navigation state
func (app *Application) updateBottomBarSafe() {
	var statusText strings.Builder
	
	// Check if we have an active confirmation message
	if time.Now().Before(app.confirmationEnd) && app.confirmationMessage != "" {
		// Show animated confirmation
		pulse := []string{"●", "◐", "◑", "◒", "◓", "○"}
		pulseFrame := (app.animationFrame / pulseCycleFrames) % len(pulse)
		
		statusText.WriteString(fmt.Sprintf(" [yellow]%s [white]%s", 
			pulse[pulseFrame], app.confirmationMessage))
	} else {
		// Show regular status
		app.confirmationMessage = ""
		
		var totalEntries int
		if app.isLoading {
			totalEntries = app.streamingLoader.GetEntryCount()
			if app.loadingProgress > 0 {
				statusText.WriteString(fmt.Sprintf("Loading... %d entries", app.loadingProgress))
			} else {
				statusText.WriteString("Starting load...")
			}
			if len(app.filteredEntries) > 0 {
				statusText.WriteString(fmt.Sprintf(" | Showing %d/%d", len(app.filteredEntries), totalEntries))
			}
		} else if app.harData != nil {
			totalEntries = len(app.harData.Log.Entries)
			statusText.WriteString(fmt.Sprintf("Showing %d/%d requests", len(app.filteredEntries), totalEntries))
		}
		
		if app.filterState.FilterText != "" {
			statusText.WriteString(fmt.Sprintf(" | Filter: [cyan]%s[white]", app.filterState.FilterText))
		}
		if app.filterState.ShowErrorsOnly {
			statusText.WriteString(" | [red]Errors Only[white]")
		}
		if app.filterState.SortBySlowest {
			statusText.WriteString(" | [yellow]Sorted by Time[white]")
		}
		if app.filterState.ActiveTypeFilter != "all" {
			statusText.WriteString(fmt.Sprintf(" | [cyan]Type: %s[white]", app.filterState.ActiveTypeFilter))
		}
	}
	
	// Add contextual information on the right side
	contextInfo := app.getContentContext()
	// Force display of context info even if it's empty for debugging
	if contextInfo != "" || true {
		if contextInfo == "" {
			contextInfo = "[dim]DEBUG: no context[white]"
		}
		// Calculate padding to right-align context info
		_, _, width, _ := app.bottomBar.GetRect()
		leftText := " " + statusText.String() + " "
		rightText := " " + contextInfo + " "
		
		// Calculate visible lengths (without color codes)
		leftLen := calculateVisibleLength(leftText)
		rightLen := calculateVisibleLength(rightText)
		
		// Always show context info, even if width calculation fails or space is tight
		if width > 0 && leftLen + rightLen < width {
			padding := width - leftLen - rightLen
			finalText := leftText + strings.Repeat(" ", padding) + rightText
			app.bottomBar.SetText(finalText)
		} else {
			// Fallback: intelligently truncate rightText to fit if needed
			maxRightWidth := width - leftLen - 2 // Leave some margin
			if maxRightWidth > 10 && rightLen > maxRightWidth {
				// Truncate context info intelligently, preserving important parts
				truncatedRight := app.truncateContextInfo(rightText, maxRightWidth)
				app.bottomBar.SetText(leftText + truncatedRight)
			} else {
				// Show both left and right with minimal spacing
				app.bottomBar.SetText(leftText + rightText)
			}
		}
	} else {
		app.bottomBar.SetText(" " + statusText.String() + " ")
	}
}



// getContentContext returns contextual information about the current content
func (app *Application) getContentContext() string {
	// Only show context when viewing content tabs and have a valid selection
	if len(app.filteredEntries) == 0 {
		return ""
	}
	
	currentIndex := app.requests.GetCurrentItem()
	if currentIndex < 0 || currentIndex >= len(app.filteredEntries) {
		return ""
	}
	
	entryIdx := app.filteredEntries[currentIndex]
	var entries []har.HAREntry
	if app.isLoading {
		entries = app.streamingLoader.GetEntries()
	} else if app.harData != nil {
		entries = app.harData.Log.Entries
	} else {
		return ""
	}
	
	if entryIdx >= len(entries) {
		return ""
	}
	
	entry := entries[entryIdx]
	
	// Generate context based on current tab
	switch app.currentTab {
	case 0: // Request tab
		return app.getRequestContext(entry)
	case 1: // Response tab  
		return app.getResponseContext(entry)
	case 2: // Body tab - always use enhanced context for consistency
		return app.getEnhancedBodyContext(entry)
	case 3: // Cookies tab
		return app.getCookiesContext(entry)
	case 4: // Timings tab
		return app.getTimingsContext(entry)
	case 5: // Raw tab
		return app.getRawContext(entry)
	default:
		return ""
	}
}

// getRequestContext returns context info for the Request tab
func (app *Application) getRequestContext(entry har.HAREntry) string {
	var context []string
	
	// Method and URL info
	context = append(context, fmt.Sprintf("[cyan]%s[white]", entry.Request.Method))
	
	// Content length if available
	if entry.Request.PostData != nil && entry.Request.PostData.Text != "" {
		size := len(entry.Request.PostData.Text)
		context = append(context, fmt.Sprintf("%s", formatBytes(size)))
	}
	
	// Header count
	headerCount := len(entry.Request.Headers)
	context = append(context, fmt.Sprintf("%d headers", headerCount))
	
	return strings.Join(context, " | ")
}

// getResponseContext returns context info for the Response tab  
func (app *Application) getResponseContext(entry har.HAREntry) string {
	var context []string
	
	// Status with color
	statusColor := "white"
	if entry.Response.Status >= 400 {
		statusColor = "red"
	} else if entry.Response.Status >= 300 {
		statusColor = "yellow"  
	} else if entry.Response.Status >= 200 {
		statusColor = "green"
	}
	context = append(context, fmt.Sprintf("[%s]%d[white]", statusColor, entry.Response.Status))
	
	// Content size
	if entry.Response.Content.Size > 0 {
		context = append(context, formatBytes(entry.Response.Content.Size))
	}
	
	// MIME type
	if entry.Response.Content.MimeType != "" {
		context = append(context, fmt.Sprintf("[dim]%s[white]", entry.Response.Content.MimeType))
	}
	
	return strings.Join(context, " | ")
}

// getEnhancedBodyContext returns detailed context info when focused on the Body tab content
func (app *Application) getEnhancedBodyContext(entry har.HAREntry) string {
	bodyText := har.DecodeBase64(entry.Response.Content.Text, entry.Response.Content.Encoding)
	if bodyText == "" {
		return "[dim]Empty body[white]"
	}
	
	contentType := app.formatter.DetectContentType(bodyText, entry.Response.Content.MimeType)
	
	switch contentType {
	case "json":
		return app.getEnhancedJSONContext(bodyText)
	case "javascript":
		return app.getEnhancedJavaScriptContext(bodyText)
	case "html":
		return app.getEnhancedHTMLContext(bodyText)
	case "css":
		return app.getEnhancedCSSContext(bodyText)
	case "image":
		return app.getEnhancedImageContext(bodyText, entry.Response.Content.MimeType)
	default:
		// Enhanced generic text context
		return app.getEnhancedTextContext(bodyText, contentType)
	}
}

// getEnhancedJSONContext returns detailed JSON analysis when focused
func (app *Application) getEnhancedJSONContext(content string) string {
	var context []string
	
	// Get current entry index for path tracking
	currentIndex := app.requests.GetCurrentItem()
	var entryIdx int = -1
	if currentIndex >= 0 && currentIndex < len(app.filteredEntries) {
		entryIdx = app.filteredEntries[currentIndex]
		
		// Build JSON path mapping (cached for performance)
		pathInfo := app.buildJSONPathMapping(content, entryIdx)
		
		// Get current JSON path when scrolling
		if app.focusOnBottom && pathInfo.isValid {
			currentPath := app.getCurrentJSONPath(entryIdx)
			if currentPath != "" {
				// Truncate very long paths for display
				if len(currentPath) > 40 {
					parts := strings.Split(currentPath, ".")
					if len(parts) > 3 {
						currentPath = strings.Join(parts[:2], ".") + "..." + strings.Join(parts[len(parts)-2:], ".")
					}
				}
				context = append(context, fmt.Sprintf("[magenta]%s[white]", currentPath))
			}
		}
	}
	
	// Always show size info first (most important and should always be visible)
	prettyContent := app.prettyPrintJSON(content)
	lines := strings.Count(prettyContent, "\n") + 1
	context = append(context, fmt.Sprintf("[green]%d lines[white]", lines))
	
	// ALWAYS add file size - this should never be conditional
	fileSize := formatBytes(len(content))
	context = append(context, fmt.Sprintf("[dim]%s[white]", fileSize))
	
	// Parse and analyze JSON structure in detail (optional, if parsing succeeds)
	var data interface{}
	if json.Unmarshal([]byte(content), &data) == nil {
		// Get current depth based on user's position, not max depth
		currentDepth := app.getCurrentJSONDepth(entryIdx)
		_, objects, arrays := app.analyzeJSONStructure(data, 0)
		context = append(context, fmt.Sprintf("[yellow]depth %d[white]", currentDepth))
		context = append(context, fmt.Sprintf("[cyan]%d objects[white]", objects))
		context = append(context, fmt.Sprintf("[blue]%d arrays[white]", arrays))
	}
	
	return strings.Join(context, " | ")
}

// getEnhancedJavaScriptContext returns detailed JS analysis when focused
func (app *Application) getEnhancedJavaScriptContext(content string) string {
	var context []string
	
	lines := strings.Count(content, "\n") + 1
	context = append(context, fmt.Sprintf("[green]%d lines[white]", lines))
	
	// Enhanced function counting
	functionCount := strings.Count(content, "function ") 
	arrowFunctionCount := strings.Count(content, "=>")
	asyncCount := strings.Count(content, "async ")
	
	if functionCount > 0 || arrowFunctionCount > 0 {
		context = append(context, fmt.Sprintf("[cyan]%d functions[white]", functionCount+arrowFunctionCount))
	}
	if asyncCount > 0 {
		context = append(context, fmt.Sprintf("[yellow]%d async[white]", asyncCount))
	}
	
	// Look for common patterns
	if strings.Contains(content, "import ") || strings.Contains(content, "export ") {
		context = append(context, "[blue]ES6 modules[white]")
	}
	if strings.Contains(content, "class ") {
		classCount := strings.Count(content, "class ")
		context = append(context, fmt.Sprintf("[magenta]%d classes[white]", classCount))
	}
	
	context = append(context, fmt.Sprintf("[dim]%s[white]", formatBytes(len(content))))
	
	return strings.Join(context, " | ")
}

// getEnhancedHTMLContext returns detailed HTML analysis when focused
func (app *Application) getEnhancedHTMLContext(content string) string {
	var context []string
	
	// Count different types of elements
	divCount := strings.Count(content, "<div")
	scriptCount := strings.Count(content, "<script")
	linkCount := strings.Count(content, "<link")
	imgCount := strings.Count(content, "<img")
	
	context = append(context, fmt.Sprintf("[cyan]%d divs[white]", divCount))
	if scriptCount > 0 {
		context = append(context, fmt.Sprintf("[yellow]%d scripts[white]", scriptCount))
	}
	if linkCount > 0 {
		context = append(context, fmt.Sprintf("[green]%d links[white]", linkCount))
	}
	if imgCount > 0 {
		context = append(context, fmt.Sprintf("[magenta]%d images[white]", imgCount))
	}
	
	// Look for title
	if strings.Contains(strings.ToLower(content), "<title>") {
		start := strings.Index(strings.ToLower(content), "<title>") + 7
		end := strings.Index(strings.ToLower(content[start:]), "</title>")
		if end > 0 && end < 50 {
			title := strings.TrimSpace(content[start : start+end])
			if title != "" {
				context = append(context, fmt.Sprintf("[dim]\"%s\"[white]", title))
			}
		}
	}
	
	lines := strings.Count(content, "\n") + 1
	context = append(context, fmt.Sprintf("[green]%d lines[white]", lines))
	context = append(context, fmt.Sprintf("[dim]%s[white]", formatBytes(len(content))))
	
	return strings.Join(context, " | ")
}

// getEnhancedCSSContext returns detailed CSS analysis when focused
func (app *Application) getEnhancedCSSContext(content string) string {
	var context []string
	
	// Count different CSS constructs
	ruleCount := strings.Count(content, "{")
	mediaCount := strings.Count(content, "@media")
	keyframesCount := strings.Count(content, "@keyframes")
	importCount := strings.Count(content, "@import")
	
	context = append(context, fmt.Sprintf("[cyan]%d rules[white]", ruleCount))
	if mediaCount > 0 {
		context = append(context, fmt.Sprintf("[yellow]%d media queries[white]", mediaCount))
	}
	if keyframesCount > 0 {
		context = append(context, fmt.Sprintf("[green]%d animations[white]", keyframesCount))
	}
	if importCount > 0 {
		context = append(context, fmt.Sprintf("[blue]%d imports[white]", importCount))
	}
	
	lines := strings.Count(content, "\n") + 1
	context = append(context, fmt.Sprintf("[magenta]%d lines[white]", lines))
	context = append(context, fmt.Sprintf("[dim]%s[white]", formatBytes(len(content))))
	
	return strings.Join(context, " | ")
}

// getEnhancedImageContext returns detailed image analysis when focused
func (app *Application) getEnhancedImageContext(content, mimeType string) string {
	// Same as regular image context but with more details
	return app.getImageContext(content, mimeType)
}

// getEnhancedTextContext returns detailed text analysis when focused
func (app *Application) getEnhancedTextContext(content, contentType string) string {
	var context []string
	
	lines := strings.Count(content, "\n") + 1
	words := len(strings.Fields(content))
	
	context = append(context, fmt.Sprintf("[green]%d lines[white]", lines))
	context = append(context, fmt.Sprintf("[cyan]%d words[white]", words))
	context = append(context, fmt.Sprintf("[dim]%s[white]", formatBytes(len(content))))
	context = append(context, fmt.Sprintf("[dim]%s[white]", contentType))
	
	return strings.Join(context, " | ")
}

// getImageContext returns context info for images
func (app *Application) getImageContext(content, mimeType string) string {
	var context []string
	
	// Try to get image info using the image displayer
	imageInfo := app.analyzeImageContent([]byte(content), mimeType)
	if imageInfo != "" {
		context = append(context, imageInfo)
	}
	
	// File size
	context = append(context, formatBytes(len(content)))
	
	// Format from content analysis, not MIME type (more reliable)
	data := []byte(content)
	format := ""
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		format = "PNG"
	} else if len(data) >= 3 && string(data[:3]) == "\xff\xd8\xff" {
		format = "JPEG"
	} else if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		format = "GIF"
	} else if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		format = "WEBP"
	} else if mimeType != "" {
		// Fallback to MIME type if content detection fails
		format = strings.ToUpper(strings.TrimPrefix(mimeType, "image/"))
	}
	
	if format != "" {
		context = append(context, fmt.Sprintf("[cyan]%s[white]", format))
	}
	
	return strings.Join(context, " | ")
}

// analyzeImageContent tries to extract image dimensions and other info
func (app *Application) analyzeImageContent(data []byte, mimeType string) string {
	if len(data) < 10 {
		return ""
	}
	
	// Basic image format detection and dimension extraction
	switch {
	case len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n":
		// PNG format - dimensions are at bytes 16-23
		if len(data) >= 24 {
			width := uint32(data[16])<<24 | uint32(data[17])<<16 | uint32(data[18])<<8 | uint32(data[19])
			height := uint32(data[20])<<24 | uint32(data[21])<<16 | uint32(data[22])<<8 | uint32(data[23])
			return fmt.Sprintf("[yellow]%dx%d[white]", width, height)
		}
		return "[yellow]PNG[white]"
	case len(data) >= 3 && string(data[:3]) == "\xff\xd8\xff":
		// JPEG format - try to find dimensions in EXIF data
		width, height := app.extractJPEGDimensions(data)
		if width > 0 && height > 0 {
			return fmt.Sprintf("[yellow]%dx%d[white]", width, height)
		}
		return "[yellow]JPEG[white]"
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		// GIF format - dimensions at bytes 6-9
		if len(data) >= 10 {
			width := uint16(data[6]) | uint16(data[7])<<8
			height := uint16(data[8]) | uint16(data[9])<<8
			return fmt.Sprintf("[yellow]%dx%d[white]", width, height)
		}
		return "[yellow]GIF[white]"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		// WebP format
		return "[yellow]WEBP[white]"
	default:
		return ""
	}
}

// extractJPEGDimensions tries to extract dimensions from JPEG data
func (app *Application) extractJPEGDimensions(data []byte) (width, height uint16) {
	// Simple JPEG dimension extraction - look for SOF0 marker
	for i := 2; i < len(data)-8; i++ {
		if data[i] == 0xFF && data[i+1] == 0xC0 { // SOF0 marker
			// Dimensions are at offset +5 and +7 from SOF0
			if i+9 < len(data) {
				height = uint16(data[i+5])<<8 | uint16(data[i+6])
				width = uint16(data[i+7])<<8 | uint16(data[i+8])
				return width, height
			}
		}
	}
	return 0, 0
}


// analyzeJSONStructure recursively analyzes JSON structure
func (app *Application) analyzeJSONStructure(data interface{}, currentDepth int) (maxDepth, objects, arrays int) {
	switch v := data.(type) {
	case map[string]interface{}:
		objects = 1
		maxDepth = currentDepth + 1 // Objects add 1 to depth
		// Recursively analyze each value in the object
		for _, value := range v {
			childDepth, childObjects, childArrays := app.analyzeJSONStructure(value, currentDepth + 1)
			if childDepth > maxDepth {
				maxDepth = childDepth
			}
			objects += childObjects
			arrays += childArrays
		}
	case []interface{}:
		arrays = 1
		maxDepth = currentDepth + 1 // Arrays add 1 to depth
		// Recursively analyze each element in the array
		for _, value := range v {
			childDepth, childObjects, childArrays := app.analyzeJSONStructure(value, currentDepth + 1)
			if childDepth > maxDepth {
				maxDepth = childDepth
			}
			objects += childObjects
			arrays += childArrays
		}
	default:
		// Primitive values (string, number, boolean, null) don't add depth
		maxDepth = currentDepth
		objects = 0
		arrays = 0
	}
	
	return maxDepth, objects, arrays
}

// getJavaScriptContext returns context info for JavaScript
func (app *Application) getJavaScriptContext(content string) string {
	var context []string
	
	lines := strings.Count(content, "\n") + 1
	context = append(context, fmt.Sprintf("%d lines", lines))
	
	// Count functions (simple regex)
	functionCount := strings.Count(content, "function ") + strings.Count(content, "() =>") + strings.Count(content, ") =>")
	if functionCount > 0 {
		context = append(context, fmt.Sprintf("%d functions", functionCount))
	}
	
	context = append(context, formatBytes(len(content)))
	context = append(context, "[cyan]JS[white]")
	
	return strings.Join(context, " | ")
}

// getHTMLContext returns context info for HTML
func (app *Application) getHTMLContext(content string) string {
	var context []string
	
	// Count elements (approximate)
	elementCount := strings.Count(content, "<") / 2 // Rough estimate
	if elementCount > 0 {
		context = append(context, fmt.Sprintf("%d elements", elementCount))
	}
	
	// Look for title
	if strings.Contains(strings.ToLower(content), "<title>") {
		start := strings.Index(strings.ToLower(content), "<title>") + 7
		end := strings.Index(strings.ToLower(content[start:]), "</title>")
		if end > 0 && end < 50 {
			title := strings.TrimSpace(content[start : start+end])
			if title != "" {
				context = append(context, fmt.Sprintf("[dim]%s[white]", title))
			}
		}
	}
	
	lines := strings.Count(content, "\n") + 1
	context = append(context, fmt.Sprintf("%d lines", lines))
	context = append(context, "[cyan]HTML[white]")
	
	return strings.Join(context, " | ")
}

// getCSSContext returns context info for CSS
func (app *Application) getCSSContext(content string) string {
	var context []string
	
	// Count rules (approximate)
	ruleCount := strings.Count(content, "{")
	if ruleCount > 0 {
		context = append(context, fmt.Sprintf("%d rules", ruleCount))
	}
	
	lines := strings.Count(content, "\n") + 1
	context = append(context, fmt.Sprintf("%d lines", lines))
	context = append(context, formatBytes(len(content)))
	context = append(context, "[cyan]CSS[white]")
	
	return strings.Join(context, " | ")
}

// getXMLContext returns context info for XML/SVG
func (app *Application) getXMLContext(content string) string {
	var context []string
	
	// Count elements
	elementCount := strings.Count(content, "<") / 2
	if elementCount > 0 {
		context = append(context, fmt.Sprintf("%d elements", elementCount))
	}
	
	// Root element
	if strings.HasPrefix(strings.TrimSpace(content), "<") {
		start := strings.Index(content, "<") + 1
		end := strings.Index(content[start:], " ")
		if end == -1 {
			end = strings.Index(content[start:], ">")
		}
		if end > 0 && end < 20 {
			rootElement := content[start : start+end]
			context = append(context, fmt.Sprintf("[dim]<%s>[white]", rootElement))
		}
	}
	
	lines := strings.Count(content, "\n") + 1
	context = append(context, fmt.Sprintf("%d lines", lines))
	context = append(context, "[cyan]XML[white]")
	
	return strings.Join(context, " | ")
}

// getCookiesContext returns context info for the Cookies tab
func (app *Application) getCookiesContext(entry har.HAREntry) string {
	reqCookies := len(entry.Request.Cookies)
	respCookies := len(entry.Response.Cookies)
	total := reqCookies + respCookies
	
	if total == 0 {
		return "[dim]No cookies[white]"
	}
	
	var context []string
	if reqCookies > 0 {
		context = append(context, fmt.Sprintf("%d request", reqCookies))
	}
	if respCookies > 0 {
		context = append(context, fmt.Sprintf("%d response", respCookies))
	}
	context = append(context, "[cyan]cookies[white]")
	
	return strings.Join(context, " | ")
}

// getTimingsContext returns context info for the Timings tab
func (app *Application) getTimingsContext(entry har.HAREntry) string {
	var context []string
	
	// Total time
	context = append(context, fmt.Sprintf("[yellow]%.1fms[white] total", entry.Time))
	
	// Slowest phase
	timings := entry.Timings
	phases := map[string]float64{
		"blocked": timings.Blocked,
		"dns":     timings.DNS,
		"connect": timings.Connect,
		"ssl":     timings.SSL,
		"send":    timings.Send,
		"wait":    timings.Wait,
		"receive": timings.Receive,
	}
	
	var slowestPhase string
	var slowestTime float64
	for phase, time := range phases {
		if time > slowestTime {
			slowestTime = time
			slowestPhase = phase
		}
	}
	
	if slowestPhase != "" && slowestTime > 0 {
		context = append(context, fmt.Sprintf("[dim]%s: %.1fms[white]", slowestPhase, slowestTime))
	}
	
	return strings.Join(context, " | ")
}

// getRawContext returns context info for the Raw tab
func (app *Application) getRawContext(entry har.HAREntry) string {
	// JSON size and structure info for the raw entry
	rawJSON := app.prettyJSON(entry)
	lines := strings.Count(rawJSON, "\n") + 1
	
	var context []string
	context = append(context, fmt.Sprintf("%d lines", lines))
	context = append(context, formatBytes(len(rawJSON)))
	context = append(context, "[cyan]HAR JSON[white]")
	
	return strings.Join(context, " | ")
}

// formatBytes formats byte count in human readable format
func formatBytes(bytes int) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	} else {
		return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
	}
}

// calculateVisibleLength calculates the visible length of a string, ignoring tview color codes
func calculateVisibleLength(s string) int {
	// Simple approach: count everything that's not a tview color tag
	result := s
	inTag := false
	visibleLen := 0
	
	for _, r := range result {
		if r == '[' {
			inTag = true
		} else if r == ']' && inTag {
			inTag = false
		} else if !inTag {
			visibleLen++
		}
	}
	
	return visibleLen
}

// truncateContextInfo intelligently truncates context information to fit available space
func (app *Application) truncateContextInfo(contextText string, maxWidth int) string {
	// Remove leading/trailing spaces for processing
	trimmed := strings.TrimSpace(contextText)
	
	// If it already fits, return as-is
	if calculateVisibleLength(trimmed) <= maxWidth {
		return contextText
	}
	
	// Split by pipe separators to prioritize parts
	parts := strings.Split(trimmed, " | ")
	if len(parts) <= 1 {
		// No separators, just truncate with ellipsis
		return app.truncateWithEllipsis(trimmed, maxWidth)
	}
	
	// Build result prioritizing important information
	var result []string
	currentLen := 0
	
	// Priority order: size info, lines, path, then others
	priorityOrder := make([]string, 0, len(parts))
	otherParts := make([]string, 0)
	
	for _, part := range parts {
		trimPart := strings.TrimSpace(part)
		if strings.Contains(trimPart, "lines") || strings.Contains(trimPart, "B") || strings.Contains(trimPart, "KB") || strings.Contains(trimPart, "MB") {
			// High priority: size and line info
			priorityOrder = append(priorityOrder, trimPart)
		} else if strings.Contains(trimPart, "arrays") || strings.Contains(trimPart, "objects") {
			// Medium priority: structural info
			priorityOrder = append(priorityOrder, trimPart)
		} else {
			// Lower priority
			otherParts = append(otherParts, trimPart)
		}
	}
	
	// Append other parts after priority ones
	priorityOrder = append(priorityOrder, otherParts...)
	
	// Build final string within width constraints
	for i, part := range priorityOrder {
		partLen := calculateVisibleLength(part)
		separatorLen := 3 // " | "
		
		if i == 0 {
			// First part, no separator needed
			if partLen <= maxWidth {
				result = append(result, part)
				currentLen = partLen
			} else {
				// Even first part doesn't fit, truncate it
				result = append(result, app.truncateWithEllipsis(part, maxWidth))
				break
			}
		} else {
			// Need separator + part
			neededLen := separatorLen + partLen
			if currentLen + neededLen <= maxWidth {
				result = append(result, part)
				currentLen += neededLen
			} else {
				// Try to fit a truncated version
				availableWidth := maxWidth - currentLen - separatorLen - 3 // Reserve 3 for "..."
				if availableWidth > 5 {
					truncatedPart := app.truncateWithEllipsis(part, availableWidth)
					result = append(result, truncatedPart)
				}
				break
			}
		}
	}
	
	return " " + strings.Join(result, " | ") + " "
}

// truncateWithEllipsis truncates text to fit width, preserving color codes
func (app *Application) truncateWithEllipsis(text string, maxWidth int) string {
	if maxWidth <= 3 {
		return "..."
	}
	
	if calculateVisibleLength(text) <= maxWidth {
		return text
	}
	
	// Build result character by character, preserving color codes
	var result strings.Builder
	visibleCount := 0
	inTag := false
	runes := []rune(text)
	
	for _, r := range runes {
		if r == '[' {
			inTag = true
			result.WriteRune(r)
		} else if r == ']' && inTag {
			inTag = false
			result.WriteRune(r)
		} else if inTag {
			result.WriteRune(r)
		} else {
			if visibleCount >= maxWidth-3 {
				result.WriteString("...")
				break
			}
			result.WriteRune(r)
			visibleCount++
		}
	}
	
	return result.String()
}

// buildJSONPathMapping creates a mapping of line numbers to JSON paths using gjson
func (app *Application) buildJSONPathMapping(content string, entryIdx int) *JSONPathInfo {
	// Check cache first
	if cached, exists := app.jsonPathCache[entryIdx]; exists {
		return cached
	}
	
	// Use pretty-printed JSON for consistent line mapping
	prettyContent := app.prettyPrintJSON(content)
	
	pathInfo := &JSONPathInfo{
		lineToPath: make(map[int]string),
		totalLines: strings.Count(prettyContent, "\n") + 1,
		isValid:    false,
	}
	
	// Validate JSON first
	if !gjson.Valid(content) {
		app.jsonPathCache[entryIdx] = pathInfo
		return pathInfo
	}
	
	pathInfo.isValid = true
	
	// Build a simple line-to-path mapping by parsing the pretty-printed JSON
	app.buildSimplePathMapping(prettyContent, pathInfo)
	
	// Cache the result
	app.jsonPathCache[entryIdx] = pathInfo
	return pathInfo
}

// buildSimplePathMapping creates accurate line-to-path mappings from pretty-printed JSON
func (app *Application) buildSimplePathMapping(prettyContent string, pathInfo *JSONPathInfo) {
	lines := strings.Split(prettyContent, "\n")
	
	// Use a simple approach: for each line, try to determine what JSON path it represents
	// by looking at its indentation and content
	
	for lineNum, line := range lines {
		lineNumber := lineNum + 1 // Convert to 1-based
		trimmed := strings.TrimSpace(line)
		
		// Skip empty lines and structural lines
		if trimmed == "" || trimmed == "{" || trimmed == "}" || trimmed == "[" || trimmed == "]" || 
		   trimmed == "{," || trimmed == "}," || trimmed == "[," || trimmed == "]," {
			continue
		}
		
		// For lines with key-value pairs, extract the path by analyzing the structure above
		if strings.Contains(trimmed, ":") {
			path := app.extractJSONPathFromContext(lines, lineNum)
			if path != "" {
				pathInfo.lineToPath[lineNumber] = path
			}
		}
	}
}

// extractJSONPathFromContext determines the JSON path for a line by analyzing surrounding context
func (app *Application) extractJSONPathFromContext(lines []string, targetLineNum int) string {
	if targetLineNum >= len(lines) {
		return ""
	}
	
	targetLine := strings.TrimSpace(lines[targetLineNum])
	if !strings.Contains(targetLine, ":") {
		return ""
	}
	
	// Extract the key from this line
	key := strings.TrimSpace(strings.Split(targetLine, ":")[0])
	key = strings.Trim(key, `"`)
	
	// Build the path by looking at the structure above this line
	var pathParts []string
	indent := app.getIndentation(lines[targetLineNum])
	
	// Walk backwards to find parent objects/arrays
	for i := targetLineNum - 1; i >= 0; i-- {
		line := lines[i]
		lineIndent := app.getIndentation(line)
		trimmed := strings.TrimSpace(line)
		
		// If we find a line with less indentation that contains a key, it's a parent
		if lineIndent < indent && strings.Contains(trimmed, ":") {
			parentKey := strings.TrimSpace(strings.Split(trimmed, ":")[0])
			parentKey = strings.Trim(parentKey, `"`)
			
			// Check if this parent is an array by looking at the value part
			valuePart := strings.TrimSpace(strings.Split(trimmed, ":")[1])
			if strings.HasPrefix(valuePart, "[") {
				// This is an array - we need to find which index we're in
				arrayIndex := app.findArrayIndex(lines, i, targetLineNum)
				pathParts = append([]string{fmt.Sprintf("%s[%d]", parentKey, arrayIndex)}, pathParts...)
			} else {
				pathParts = append([]string{parentKey}, pathParts...)
			}
			indent = lineIndent
		}
	}
	
	// Add the current key
	pathParts = append(pathParts, key)
	
	return strings.Join(pathParts, ".")
}

// getIndentation counts the indentation level of a line
func (app *Application) getIndentation(line string) int {
	count := 0
	for _, char := range line {
		if char == ' ' {
			count++
		} else if char == '\t' {
			count += 4 // Treat tab as 4 spaces
		} else {
			break
		}
	}
	return count
}

// findArrayIndex determines which array index a target line represents
func (app *Application) findArrayIndex(lines []string, arrayStartLine, targetLine int) int {
	arrayIndent := app.getIndentation(lines[arrayStartLine])
	index := 0
	elementsSeen := 0
	
	// Look for array element starts (objects "{" or direct values)
	for i := arrayStartLine + 1; i < len(lines); i++ {
		line := lines[i]
		lineIndent := app.getIndentation(line)
		trimmed := strings.TrimSpace(line)
		
		// Skip empty lines and closing brackets
		if trimmed == "" || strings.HasPrefix(trimmed, "]") {
			continue
		}
		
		// Look for new array elements (more indented than array, starting new elements)
		if lineIndent > arrayIndent {
			// This is a new array element if it's an object start or first key of an element
			if strings.HasPrefix(trimmed, "{") || 
			   (strings.Contains(trimmed, ":") && !strings.HasSuffix(trimmed, "[") && 
			    !strings.HasSuffix(trimmed, "{") && app.isFirstKeyOfArrayElement(lines, i, arrayIndent)) {
				
				// If this line is at or before our target, this element contains our target
				if i <= targetLine {
					index = elementsSeen
				}
				elementsSeen++
				
				// If we've passed the target line, we have our answer
				if i >= targetLine {
					break
				}
			}
		}
	}
	
	return index
}

// isFirstKeyOfArrayElement checks if a line is the first key of an array element
func (app *Application) isFirstKeyOfArrayElement(lines []string, lineIndex int, arrayIndent int) bool {
	currentIndent := app.getIndentation(lines[lineIndex])
	
	// Look backwards to see if there's an object start at this indent level recently
	for i := lineIndex - 1; i >= 0; i-- {
		line := lines[i]
		indent := app.getIndentation(line)
		trimmed := strings.TrimSpace(line)
		
		// If we hit the array level, we've gone too far
		if indent <= arrayIndent {
			break
		}
		
		// If we find an object start at this level, this is likely the first key
		if indent == currentIndent && strings.HasPrefix(trimmed, "{") {
			return true
		}
		
		// If we find another key at this level, this is not the first key
		if indent == currentIndent && strings.Contains(trimmed, ":") {
			return false
		}
	}
	
	return false
}

// getCurrentJSONDepth calculates the depth of the current user position in JSON
func (app *Application) getCurrentJSONDepth(entryIdx int) int {
	// Get the current JSON path (like "icons[0].src")
	currentPath := app.getCurrentJSONPath(entryIdx)
	if currentPath == "" {
		return 1 // Root level (on { or root level keys)
	}
	
	// Root level keys should always be depth 1, regardless of what path is generated
	// Count dots and brackets to determine nesting level
	dotCount := strings.Count(currentPath, ".")
	bracketCount := strings.Count(currentPath, "[")
	
	// If this is a simple key name with no dots or brackets, it's depth 1
	// even if the path generation added extra levels
	if dotCount == 0 && bracketCount == 0 {
		return 1
	}
	
	// For truly nested paths, calculate depth based on actual nesting
	return 1 + dotCount + bracketCount
}

// walkJSONForPaths recursively walks JSON structure to build line->path mappings
func (app *Application) walkJSONForPaths(result gjson.Result, currentPath string, currentLine int, pathInfo *JSONPathInfo, content string) int {
	lines := strings.Split(content, "\n")
	
	switch result.Type {
	case gjson.JSON:
		if result.IsObject() {
			lineNum := currentLine
			pathInfo.lineToPath[lineNum] = currentPath
			
			result.ForEach(func(key, value gjson.Result) bool {
				keyPath := currentPath
				if keyPath != "" {
					keyPath += "."
				}
				keyPath += key.String()
				
				// Find the line where this key appears in the formatted content
				keyLine := app.findKeyInLines(lines, key.String(), lineNum)
				if keyLine != -1 {
					pathInfo.lineToPath[keyLine] = keyPath
					lineNum = keyLine + 1
				}
				
				// Recursively process the value
				lineNum = app.walkJSONForPaths(value, keyPath, lineNum, pathInfo, content)
				return true
			})
			return lineNum
		} else if result.IsArray() {
			lineNum := currentLine
			pathInfo.lineToPath[lineNum] = currentPath
			
			result.ForEach(func(key, value gjson.Result) bool {
				indexPath := fmt.Sprintf("%s[%s]", currentPath, key.String())
				
				// Find the line where this array element appears
				elemLine := app.findArrayElementInLines(lines, key.String(), lineNum)
				if elemLine != -1 {
					pathInfo.lineToPath[elemLine] = indexPath
					lineNum = elemLine + 1
				}
				
				// Recursively process the value
				lineNum = app.walkJSONForPaths(value, indexPath, lineNum, pathInfo, content)
				return true
			})
			return lineNum
		}
	}
	
	// For primitive values, just map the current line
	pathInfo.lineToPath[currentLine] = currentPath
	return currentLine + 1
}

// findKeyInLines finds the line number where a specific JSON key appears
func (app *Application) findKeyInLines(lines []string, key string, startLine int) int {
	quotedKey := fmt.Sprintf(`"%s"`, key)
	for i := startLine; i < len(lines); i++ {
		if strings.Contains(lines[i], quotedKey) {
			return i + 1 // Convert to 1-based line numbers
		}
	}
	return -1
}

// findArrayElementInLines finds the line number where an array element appears
func (app *Application) findArrayElementInLines(lines []string, index string, startLine int) int {
	// This is a simplified approach - in practice, we'd need more sophisticated parsing
	// to accurately map array elements to line numbers in formatted JSON
	return startLine + 1
}

// getCurrentJSONPath returns the JSON path for the current highlighted line
func (app *Application) getCurrentJSONPath(entryIdx int) string {
	pathInfo := app.jsonPathCache[entryIdx]
	if pathInfo == nil || !pathInfo.isValid {
		return ""
	}
	
	// Use the current highlighted line if we're viewing JSON
	if app.isViewingJSON() && app.currentJSONLine > 0 {
		// Look for the closest path mapping at or before current line
		for line := app.currentJSONLine; line >= 1; line-- {
			if path, exists := pathInfo.lineToPath[line]; exists && path != "" {
				return path
			}
		}
	}
	
	return ""
}

// prettyPrintJSON formats JSON with proper indentation for line-by-line highlighting
func (app *Application) prettyPrintJSON(content string) string {
	var jsonData interface{}
	if json.Unmarshal([]byte(content), &jsonData) == nil {
		formatted, err := json.MarshalIndent(jsonData, "", "  ")
		if err != nil {
			return content // Return original if formatting fails
		}
		return string(formatted)
	}
	return content // Return original if not valid JSON
}

// formatJSONWithHighlight formats JSON content with line highlighting for navigation
func (app *Application) formatJSONWithHighlight(content string, entryIdx int) string {
	// Skip gjson validation since content may have syntax highlighting codes
	lines := strings.Split(content, "\n")
	app.jsonTotalLines = len(lines)
	
	// Initialize current line if not set or entry changed
	if app.currentJSONEntry != entryIdx {
		app.currentJSONEntry = entryIdx
		app.currentJSONLine = 1
	}
	
	// Ensure current line is within bounds
	if app.currentJSONLine < 1 {
		app.currentJSONLine = 1
	} else if app.currentJSONLine > app.jsonTotalLines {
		app.currentJSONLine = app.jsonTotalLines
	}
	
	// Build content with highlighting
	var result strings.Builder
	
	for i, line := range lines {
		lineNum := i + 1
		if lineNum == app.currentJSONLine {
			// Highlight current line with blue background (like the requests list selection)
			result.WriteString(fmt.Sprintf(`[white:blue]%s[-:-]`, line))
		} else {
			// Normal line
			result.WriteString(line)
		}
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}
	
	return result.String()
}

// isViewingJSON checks if we're currently viewing JSON content in the Body tab
func (app *Application) isViewingJSON() bool {
	if app.currentTab != 2 { // Not on Body tab
		return false
	}
	
	if len(app.filteredEntries) == 0 {
		return false
	}
	
	currentIndex := app.requests.GetCurrentItem()
	if currentIndex < 0 || currentIndex >= len(app.filteredEntries) {
		return false
	}
	
	entryIdx := app.filteredEntries[currentIndex]
	var entries []har.HAREntry
	if app.isLoading {
		entries = app.streamingLoader.GetEntries()
	} else if app.harData != nil {
		entries = app.harData.Log.Entries
	} else {
		return false
	}
	
	if entryIdx >= len(entries) {
		return false
	}
	
	entry := entries[entryIdx]
	bodyText := har.DecodeBase64(entry.Response.Content.Text, entry.Response.Content.Encoding)
	if bodyText == "" {
		return false
	}
	
	contentType := app.formatter.DetectContentType(bodyText, entry.Response.Content.MimeType)
	return contentType == "json"
}

// moveJSONLine moves the highlighted line in JSON content
func (app *Application) moveJSONLine(direction int) {
	if !app.isViewingJSON() {
		return
	}
	
	newLine := app.currentJSONLine + direction
	if newLine < 1 {
		newLine = 1
	} else if newLine > app.jsonTotalLines {
		newLine = app.jsonTotalLines
	}
	
	if newLine != app.currentJSONLine {
		app.currentJSONLine = newLine
		// Refresh the body content with new highlighting
		app.refreshJSONContent()
		// Update bottom bar to show new path
		app.updateBottomBar()
	}
}

// refreshJSONContent refreshes the JSON content display with current highlighting
func (app *Application) refreshJSONContent() {
	currentIndex := app.requests.GetCurrentItem()
	if currentIndex < 0 || currentIndex >= len(app.filteredEntries) {
		return
	}
	
	entryIdx := app.filteredEntries[currentIndex]
	var entries []har.HAREntry
	if app.isLoading {
		entries = app.streamingLoader.GetEntries()
	} else if app.harData != nil {
		entries = app.harData.Log.Entries
	} else {
		return
	}
	
	if entryIdx >= len(entries) {
		return
	}
	
	entry := entries[entryIdx]
	bodyText := har.DecodeBase64(entry.Response.Content.Text, entry.Response.Content.Encoding)
	if bodyText == "" {
		return
	}
	
	contentType := app.formatter.DetectContentType(bodyText, entry.Response.Content.MimeType)
	if contentType == "json" {
		// Pretty-print the JSON first so we have multiple lines to highlight
		prettyJSON := app.prettyPrintJSON(bodyText)
		// Apply syntax highlighting first
		syntaxHighlighted := app.formatter.FormatContent(prettyJSON, contentType)
		// Then apply line highlighting to the syntax-highlighted content
		finalText := app.formatJSONWithHighlight(syntaxHighlighted, entryIdx)
		app.bodyView.SetText(finalText)
		
		// Ensure the highlighted line is visible by scrolling to it
		app.scrollToJSONLine()
	}
}

// scrollToJSONLine ensures the currently highlighted JSON line is visible
func (app *Application) scrollToJSONLine() {
	if !app.isViewingJSON() || app.currentJSONLine <= 0 {
		return
	}
	
	// Get current view dimensions
	_, _, _, height := app.bodyView.GetInnerRect()
	viewHeight := height
	if viewHeight <= 0 {
		viewHeight = 20 // Default fallback
	}
	
	// Calculate desired scroll position to center the highlighted line
	desiredTop := app.currentJSONLine - (viewHeight / 2)
	if desiredTop < 0 {
		desiredTop = 0
	} else if desiredTop > app.jsonTotalLines-viewHeight {
		desiredTop = app.jsonTotalLines - viewHeight
		if desiredTop < 0 {
			desiredTop = 0
		}
	}
	
	// Scroll to position to make highlighted line visible
	app.bodyView.ScrollTo(desiredTop, 0)
}

// updateFocusStyles updates the focus styling with blinking arrows
func (app *Application) updateFocusStyles() {
	arrow := app.getBlinkingArrows()
	
	if app.focusOnBottom {
		// Top panel unfocused - update the appropriate view in topPanel
		if app.showWaterfall {
			app.waterfallView.SetBorderColor(tcell.ColorDarkGray)
			app.waterfallView.SetTitle(" 🌊 Waterfall View ")
		} else {
			app.requests.SetBorderColor(tcell.ColorDarkGray)
			app.requests.SetTitle(" 🌐 HTTP Requests ")
		}
		
		// Bottom panel focused - add blinking arrow to current tab
		tabNames := []string{"Request", "Response", "Body", "Cookies", "Timings", "Raw"}
		views := []tview.Primitive{app.requestView, app.responseView, app.bodyView, app.cookiesView, app.timingsView, app.rawView}
		colors := []tcell.Color{tcell.ColorDarkCyan, tcell.ColorDarkGreen, tcell.ColorDarkBlue, tcell.ColorDarkMagenta, tcell.ColorDarkRed, tcell.ColorYellow}
		
		for i, view := range views {
			// Handle Body tab specially when it's in side-by-side mode
			if i == 2 && app.isSideBySide && app.bodyFlexContainer != nil { // Body tab index is 2
				if i == app.currentTab {
					app.bodyFlexContainer.SetBorderColor(tcell.ColorWhite)
					app.bodyFlexContainer.SetTitle(fmt.Sprintf(" [yellow]%s[white] %s %s ", 
						arrow, "📄", "Body"))
				} else {
					app.bodyFlexContainer.SetBorderColor(tcell.ColorDarkBlue)
					app.bodyFlexContainer.SetTitle(" 📄 Body ")
				}
			} else if textView, ok := view.(*tview.TextView); ok {
				if i == app.currentTab {
					textView.SetBorderColor(tcell.ColorWhite)
					textView.SetTitle(fmt.Sprintf(" [yellow]%s[white] %s %s ", 
						arrow, []string{"📋", "📨", "📄", "🍪", "⏱️", "🔍"}[i], tabNames[i]))
				} else {
					textView.SetBorderColor(colors[i])
					textView.SetTitle(" " + []string{"📋", "📨", "📄", "🍪", "⏱️", "🔍"}[i] + " " + tabNames[i] + " ")
				}
			}
		}
	} else {
		// Top panel focused - add blinking arrow to appropriate view
		if app.showWaterfall {
			app.waterfallView.SetBorderColor(tcell.ColorTeal)
			app.waterfallView.SetTitle(fmt.Sprintf(" [cyan]%s[white] 🌊 Waterfall View ", arrow))
			app.requests.SetBorderColor(tcell.ColorDarkGray)
			app.requests.SetTitle(" 🌐 HTTP Requests ")
		} else {
			app.requests.SetBorderColor(tcell.ColorTeal)
			app.requests.SetTitle(fmt.Sprintf(" [cyan]%s[white] 🌐 HTTP Requests ", arrow))
			app.waterfallView.SetBorderColor(tcell.ColorDarkGray)
			app.waterfallView.SetTitle(" 🌊 Waterfall View ")
		}
		
		// Bottom panel unfocused - no arrows
		tabNames := []string{"Request", "Response", "Body", "Cookies", "Timings", "Raw"}
		views := []tview.Primitive{app.requestView, app.responseView, app.bodyView, app.cookiesView, app.timingsView, app.rawView}
		colors := []tcell.Color{tcell.ColorDarkCyan, tcell.ColorDarkGreen, tcell.ColorDarkBlue, tcell.ColorDarkMagenta, tcell.ColorDarkRed, tcell.ColorYellow}
		
		for i, view := range views {
			// Handle Body tab specially when it's in side-by-side mode
			if i == 2 && app.isSideBySide && app.bodyFlexContainer != nil { // Body tab index is 2
				app.bodyFlexContainer.SetBorderColor(tcell.ColorDarkBlue)
				app.bodyFlexContainer.SetTitle(" 📄 Body ")
			} else if textView, ok := view.(*tview.TextView); ok {
				textView.SetBorderColor(colors[i])
				textView.SetTitle(" " + []string{"📋", "📨", "📄", "🍪", "⏱️", "🔍"}[i] + " " + tabNames[i] + " ")
			}
		}
	}
}

// restoreNormalBodyView ensures the Body tab is using the normal TextView (not side-by-side layout)
func (app *Application) restoreNormalBodyView() {
	// Store current focus to restore it after tab modification
	currentFocus := app.app.GetFocus()
	
	// Simple approach: Always ensure the Body tab has the normal TextView
	// Remove and re-add the Body tab with the normal body view
	// This is safe because we only call this when we want normal content
	app.tabs.RemovePage("Body")
	app.tabs.AddPage("Body", app.bodyView, true, app.currentTab == 2)
	
	// Restore focus to prevent input handling issues
	if currentFocus != nil {
		app.app.SetFocus(currentFocus)
	}
	
	// Clear side-by-side state
	app.isSideBySide = false
	app.sideBySideViews[0] = nil
	app.sideBySideViews[1] = nil
	app.bodyFlexContainer = nil
}

// setupSideBySideBodyView creates a native tview layout for side-by-side content display
func (app *Application) setupSideBySideBodyView(markerContent string) {
	// Parse the TVIEW_LAYOUT marker format: "TVIEW_LAYOUT:leftTitle|||SPLIT|||rightTitle|||SPLIT|||leftContent|||SPLIT|||rightContent"
	if !strings.HasPrefix(markerContent, "TVIEW_LAYOUT:") {
		// Fallback to regular display if marker is invalid
		app.bodyView.SetText(markerContent)
		return
	}
	
	// Extract content from marker
	content := strings.TrimPrefix(markerContent, "TVIEW_LAYOUT:")
	parts := strings.Split(content, "|||SPLIT|||")
	if len(parts) != 4 {
		// Fallback to regular display if parsing fails
		app.bodyView.SetText(fmt.Sprintf("[red]Error: Invalid side-by-side layout format (got %d parts, expected 4)[white]", len(parts)))
		return
	}
	
	leftTitle := parts[0]
	rightTitle := parts[1]
	leftContent := parts[2]
	rightContent := parts[3]
	
	// Create left and right text views
	leftView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false).
		SetScrollable(true)
	leftView.SetBorder(true).SetTitle(" " + leftTitle + " ")
	leftView.SetText(leftContent)
	
	rightView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false).
		SetScrollable(true)
	rightView.SetBorder(true).SetTitle(" " + rightTitle + " ")
	rightView.SetText(rightContent)
	
	// Store references for scrolling support
	app.sideBySideViews[0] = leftView  // Left pane (image/SVG preview)
	app.sideBySideViews[1] = rightView // Right pane (hex data/SVG code) 
	app.isSideBySide = true
	
	// Create a flex layout to hold both views side by side
	sideBySideFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
	sideBySideFlex.AddItem(leftView, 0, 1, false)   // Left panel takes half the space
	sideBySideFlex.AddItem(rightView, 0, 1, false)  // Right panel takes half the space
	
	// Set initial border and title for the flex container
	sideBySideFlex.SetBorder(true).SetTitleAlign(tview.AlignCenter)
	
	// Store reference to the flex container for focus styling
	app.bodyFlexContainer = sideBySideFlex
	
	// Replace the body view with the side-by-side layout
	// We need to find the body view in the tabs and replace it
	app.replaceTabbedView("Body", sideBySideFlex)
}

// replaceTabbedView replaces a specific tab's content with new content
func (app *Application) replaceTabbedView(tabName string, newContent tview.Primitive) {
	// Get the current tab structure and replace the body tab specifically
	// Since we know the body tab is at index 2 (Request=0, Response=1, Body=2, etc.)
	if app.currentTab == 2 { // Body tab
		// Store current focus to restore it after tab modification
		currentFocus := app.app.GetFocus()
		
		// We need to update the tabs structure
		// Remove and re-add the Body tab with new content
		app.tabs.RemovePage("Body")
		app.tabs.AddPage("Body", newContent, true, app.currentTab == 2)
		
		// Restore focus to prevent input handling issues
		if currentFocus != nil {
			app.app.SetFocus(currentFocus)
		}
		
		// Update focus styling for the new component if it's currently focused
		if app.focusOnBottom && app.currentTab == 2 {
			if flex, ok := newContent.(*tview.Flex); ok {
				// Set border color for the flex container
				flex.SetBorderColor(tcell.ColorWhite)
				flex.SetTitle(fmt.Sprintf(" [yellow]%s[white] %s %s ", 
					app.getBlinkingArrows(), "📄", "Body"))
			}
		}
	}
}