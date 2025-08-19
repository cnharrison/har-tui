package ui

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
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
		if status >= statusCodeClientError {
			statusColor = "red"
		} else if status >= statusCodeRedirect {
			statusColor = "yellow"
		} else if status >= statusCodeSuccess {
			statusColor = "green"
		}
		
		displayText := fmt.Sprintf("[cyan]%-4s[white] [%s]%3d[white] [blue]%s[white] [dim]%s[white] [yellow]%s[white]", 
			method, statusColor, status, host, path, duration)
		
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
		}
		app.updateTabContent(app.requests.GetCurrentItem())
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
	if entry.Response.Status >= statusCodeClientError {
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
		bodyText = app.formatter.FormatContent(bodyText, contentType)
	} else {
		bodyText = "[dim]No body content[white]"
	}
	app.bodyView.SetText(bodyText)
	
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
	
	app.bottomBar.SetText(" " + statusText.String() + " ")
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
			if textView, ok := view.(*tview.TextView); ok {
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
			if textView, ok := view.(*tview.TextView); ok {
				textView.SetBorderColor(colors[i])
				textView.SetTitle(" " + []string{"📋", "📨", "📄", "🍪", "⏱️", "🔍"}[i] + " " + tabNames[i] + " ")
			}
		}
	}
}