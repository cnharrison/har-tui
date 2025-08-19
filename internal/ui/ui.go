package ui

import (
	"fmt"
	"strings"
	"time"
	"net/url"
	"net"
	"encoding/json"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/cnharrison/har-tui/internal/har"
	"github.com/cnharrison/har-tui/internal/filter"
	"github.com/cnharrison/har-tui/internal/format"
)

// Application represents the main HAR TUI application
type Application struct {
	harData     *har.HARFile
	filename    string
	app         *tview.Application
	filterState *filter.FilterState
	formatter   *format.ContentFormatter
	
	// Streaming components
	streamingLoader *har.StreamingLoader
	isLoading       bool
	loadingProgress int
	lastUpdateCount int
	batchUpdateSize int
	
	// UI state
	currentTab      int
	filteredEntries []int
	focusOnBottom   bool
	animationFrame  int
	selectedFilterIndex int
	
	// Confirmation/status messages
	confirmationMessage string
	confirmationEnd     time.Time
	
	// UI components
	requests    *tview.List
	filterBar   *tview.TextView
	topPanel    *tview.Pages  // Switches between requests list and waterfall view
	tabs        *tview.Pages
	requestView *tview.TextView
	responseView *tview.TextView
	bodyView    *tview.TextView
	cookiesView *tview.TextView
	timingsView *tview.TextView
	rawView     *tview.TextView
	waterfallView *WaterfallView
	topBar      *tview.TextView
	tabBar      *tview.TextView
	bottomBar   *tview.TextView
	searchInput *tview.InputField
	layout      *tview.Flex
	
	// UI state for top panel
	showWaterfall bool
}

// NewApplication creates a new HAR TUI application
func NewApplication(harData *har.HARFile, filename string) *Application {
	// Initialize components
	app := &Application{
		harData:     harData,
		filename:    filename,
		app:         tview.NewApplication(),
		filterState: filter.NewFilterState(),
		formatter:   format.NewContentFormatter(),
		currentTab:  0,
		filteredEntries: make([]int, 0),
		focusOnBottom: false,
		selectedFilterIndex: 0,
		streamingLoader: har.NewStreamingLoader(),
		isLoading: false,
		loadingProgress: 0,
	}
	
	// Initialize filtered entries for existing data
	if harData != nil {
		for i := range harData.Log.Entries {
			app.filteredEntries = append(app.filteredEntries, i)
		}
	}
	
	return app
}

// NewApplicationStreaming creates a new HAR TUI application with streaming loader
func NewApplicationStreaming(filename string) *Application {
	app := &Application{
		filename:    filename,
		app:         tview.NewApplication(),
		filterState: filter.NewFilterState(),
		formatter:   format.NewContentFormatter(),
		currentTab:  0,
		filteredEntries: make([]int, 0),
		focusOnBottom: false,
		selectedFilterIndex: 0,
		streamingLoader: har.NewStreamingLoader(),
		isLoading: true,
		loadingProgress: 0,
		lastUpdateCount: 0,
		batchUpdateSize: 100,
	}
	
	// Set up streaming callbacks
	app.streamingLoader.SetCallbacks(
		app.onEntryAdded,
		app.onLoadingComplete,
		app.onLoadingError,
		app.onLoadingProgress,
	)
	
	return app
}

// Run starts the TUI application
func (app *Application) Run() error {
	app.setupUI()
	app.setupEventHandling()
	app.startAnimationLoop()
	
	// Start streaming load if needed
	if app.isLoading {
		app.streamingLoader.LoadHARFileStreaming(app.filename)
	}
	
	// Initialize display
	app.updateRequestsList()
	app.updateFocusStyles()
	app.updateFilterBar()
	app.updateTabBar()
	app.updateBottomBar()
	
	return app.app.SetRoot(app.layout, true).Run()
}

// setupUI creates and configures all UI components
func (app *Application) setupUI() {
	// Configure tview for transparent background
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorDefault
	tview.Styles.ContrastBackgroundColor = tcell.ColorDefault
	
	// Create main components
	app.createComponents()
	app.styleComponents()
	app.createLayout()
}

// createComponents initializes all UI components
func (app *Application) createComponents() {
	// Top menu bar
	app.topBar = tview.NewTextView().
		SetText("[::b][yellow] 🐱 HAR TUI DELUXE - Press ? for Help [white]").
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	
	// Filter buttons bar
	app.filterBar = tview.NewTextView()
	app.filterBar.SetDynamicColors(true)
	app.filterBar.SetTextAlign(tview.AlignCenter)
	app.filterBar.SetBorder(false)
	
	// Search input
	app.searchInput = tview.NewInputField()
	app.searchInput.SetLabel("")
	app.searchInput.SetText(app.filterState.FilterText)
	app.searchInput.SetFieldWidth(0)
	app.searchInput.SetBorder(true)
	app.searchInput.SetTitle(" 🔍 Search ")
	app.searchInput.SetTitleAlign(tview.AlignCenter)
	app.searchInput.SetBorderColor(tcell.ColorGreen)
	
	// Requests list
	app.requests = tview.NewList().ShowSecondaryText(false)
	
	// Tab content views
	app.requestView = tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetWordWrap(true)
	app.responseView = tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetWordWrap(true)
	app.bodyView = tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetWordWrap(true)
	app.cookiesView = tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetWordWrap(true)
	app.timingsView = tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetWordWrap(true)
	app.rawView = tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetWordWrap(true)
	app.waterfallView = NewWaterfallView()
	app.waterfallView.SetSelectionChangedFunc(func(entryIndex int) {
		if entryIndex >= 0 {
			// Find the position in filteredEntries that corresponds to this entry index
			for i, idx := range app.filteredEntries {
				if idx == entryIndex {
					app.updateTabContent(i)
					break
				}
			}
		}
	})
	
	// Tab pages
	app.tabs = tview.NewPages()
	app.tabs.AddPage("Request", app.requestView, true, true)
	app.tabs.AddPage("Response", app.responseView, true, false)
	app.tabs.AddPage("Body", app.bodyView, true, false)
	app.tabs.AddPage("Cookies", app.cookiesView, true, false)
	app.tabs.AddPage("Timings", app.timingsView, true, false)
	app.tabs.AddPage("Raw", app.rawView, true, false)
	
	// Tab indicator bar
	app.tabBar = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
	
	// Status/bottom bar
	app.bottomBar = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
}

// styleComponents applies styling to all components
func (app *Application) styleComponents() {
	// Style the requests list
	app.requests.SetBorder(true).SetTitle(" 🌐 HTTP Requests ").SetTitleAlign(tview.AlignCenter).SetBorderColor(tcell.ColorTeal)
	app.requests.SetSelectedBackgroundColor(tcell.ColorDarkBlue)
	app.requests.SetSelectedTextColor(tcell.ColorYellow)
	app.requests.SetMainTextColor(tcell.ColorWhite)
	
	// Style the tab views with deluxe borders
	app.requestView.SetBorder(true).SetTitle(" 📋 Request ").SetTitleAlign(tview.AlignCenter).SetBorderColor(tcell.ColorDarkCyan)
	app.responseView.SetBorder(true).SetTitle(" 📨 Response ").SetTitleAlign(tview.AlignCenter).SetBorderColor(tcell.ColorDarkGreen)
	app.bodyView.SetBorder(true).SetTitle(" 📄 Body ").SetTitleAlign(tview.AlignCenter).SetBorderColor(tcell.ColorDarkBlue)
	app.cookiesView.SetBorder(true).SetTitle(" 🍪 Cookies ").SetTitleAlign(tview.AlignCenter).SetBorderColor(tcell.ColorDarkMagenta)
	app.timingsView.SetBorder(true).SetTitle(" ⏱️  Timings ").SetTitleAlign(tview.AlignCenter).SetBorderColor(tcell.ColorDarkRed)
	app.waterfallView.SetBorder(true).SetTitle(" 🌊 Waterfall ").SetTitleAlign(tview.AlignCenter).SetBorderColor(tcell.ColorDarkCyan)
	app.rawView.SetBorder(true).SetTitle(" 🔍 Raw ").SetTitleAlign(tview.AlignCenter).SetBorderColor(tcell.ColorYellow)
}

// createLayout builds the main application layout
func (app *Application) createLayout() {
	// Create centered search container
	searchContainer := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).              // Left spacer
		AddItem(app.searchInput, 0, 2, false).  // Search input (2x width)
		AddItem(nil, 0, 1, false)               // Right spacer
	
	// Create top panel that can switch between requests list and waterfall view
	app.topPanel = tview.NewPages()
	app.topPanel.AddPage("requests", app.requests, true, true)
	app.topPanel.AddPage("waterfall", app.waterfallView, true, false)
	
	// Create requests panel with filter bar and search
	requestsPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(app.filterBar, 1, 0, false).
		AddItem(searchContainer, 3, 0, false).  // Height 3 for the bordered search box
		AddItem(app.topPanel, 0, 1, true)
	
	// Main layout
	app.layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(app.topBar, 1, 0, false).
		AddItem(requestsPanel, 0, 1, true).
		AddItem(app.tabBar, 1, 0, false).
		AddItem(app.tabs, 0, 2, false).
		AddItem(app.bottomBar, 1, 0, false)
}

// setupEventHandling configures all event handlers
func (app *Application) setupEventHandling() {
	// Set up search input handlers
	app.searchInput.SetChangedFunc(func(text string) {
		app.filterState.SetTextFilter(text)
		app.updateRequestsList()
		app.updateBottomBar()
	})
	
	app.searchInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape || key == tcell.KeyEnter {
			app.searchInput.SetTitle(" 🔍 Search ")
			app.focusOnBottom = false
			app.updateFocusStyles()
			app.app.SetFocus(app.requests)
		}
	})
	
	app.searchInput.SetFocusFunc(func() {
		app.searchInput.SetTitle(" 🔍 Searching... ")
		app.searchInput.SetBorderColor(tcell.ColorYellow)
	})
	
	app.searchInput.SetBlurFunc(func() {
		app.searchInput.SetTitle(" 🔍 Search ")
		app.searchInput.SetBorderColor(tcell.ColorGreen)
	})
	
	// Set up request selection handler
	app.requests.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		app.updateTabContent(index)
	})
	
	// Set main input capture
	app.app.SetInputCapture(app.handleInput)
}

// startAnimationLoop starts the animation loop for focus arrows and status messages
func (app *Application) startAnimationLoop() {
	go func() {
		for {
			time.Sleep(500 * time.Millisecond)
			app.animationFrame++
			app.app.QueueUpdateDraw(func() {
				app.updateFocusStyles()
				app.updateBottomBar()
			})
		}
	}()
}

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
		if len(path) > 50 {
			path = path[:47] + "..."
		}
		
		method := entry.Request.Method
		status := entry.Response.Status
		duration := fmt.Sprintf("%.0fms", entry.Time)
		
		// Color code by status
		statusColor := "white"
		if status >= 400 {
			statusColor = "red"
		} else if status >= 300 {
			statusColor = "yellow"
		} else if status >= 200 {
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
	if entry.Response.Status >= 400 {
		statusColor = "red"
	} else if entry.Response.Status >= 300 {
		statusColor = "yellow"
	} else if entry.Response.Status >= 200 {
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
		pulseFrame := (app.animationFrame / 2) % len(pulse)
		remaining := app.confirmationEnd.Sub(time.Now()).Seconds()
		
		statusText.WriteString(fmt.Sprintf(" [yellow]%s [white]%s [cyan](%.0fs remaining)[white]", 
			pulse[pulseFrame], app.confirmationMessage, remaining))
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

// getBlinkingArrows returns blinking arrow characters
func (app *Application) getBlinkingArrows() string {
	if app.animationFrame%4 < 2 {
		return "►"
	}
	return " "
}

// getCurrentView returns the currently active text view for scrolling
func (app *Application) getCurrentView() *tview.TextView {
	views := []*tview.TextView{app.requestView, app.responseView, app.bodyView, app.cookiesView, app.timingsView, app.rawView}
	if app.currentTab >= 0 && app.currentTab < len(views) {
		return views[app.currentTab]
	}
	return app.requestView
}

// showStatusMessage shows a temporary status message
func (app *Application) showStatusMessage(msg string) {
	app.confirmationMessage = msg
	app.confirmationEnd = time.Now().Add(5 * time.Second)
}

// prettyJSON formats data as pretty JSON
func (app *Application) prettyJSON(data interface{}) string {
	if data == nil {
		return "[dim]null[white]"
	}
	pretty, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("[red]Error formatting JSON: %v[white]", err)
	}
	return string(pretty)
}

// extractIP extracts IP address from URL
func (app *Application) extractIP(urlStr string) string {
	if u, err := url.Parse(urlStr); err == nil {
		host := u.Host
		if strings.Contains(host, ":") {
			host, _, _ = net.SplitHostPort(host)
		}
		if net.ParseIP(host) != nil {
			return host
		}
	}
	return ""
}

// formatTimings formats timing information with visual bars
func (app *Application) formatTimings(timings har.HARTimings, totalTime float64) string {
	var result strings.Builder
	result.WriteString("[yellow]Timing Breakdown:[white]\n\n")
	
	phases := []struct {
		name  string
		value float64
		color string
	}{
		{"🚫 Blocked", timings.Blocked, "red"},
		{"🔍 DNS Lookup", timings.DNS, "blue"},
		{"🔗 Connect", timings.Connect, "green"},
		{"🔒 SSL/TLS", timings.SSL, "magenta"},
		{"📤 Send", timings.Send, "cyan"},
		{"⏳ Wait", timings.Wait, "yellow"},
		{"📥 Receive", timings.Receive, "white"},
	}
	
	maxWidth := 40
	for _, phase := range phases {
		if phase.value > 0 {
			percentage := (phase.value / totalTime) * 100
			barWidth := int((phase.value / totalTime) * float64(maxWidth))
			if barWidth < 1 && phase.value > 0 {
				barWidth = 1
			}
			
			bar := strings.Repeat("█", barWidth) + strings.Repeat("░", maxWidth-barWidth)
			result.WriteString(fmt.Sprintf("%-12s [%s]%s[white] %.2fms (%.1f%%)\n", 
				phase.name, phase.color, bar, phase.value, percentage))
		}
	}
	
	result.WriteString(fmt.Sprintf("\n[yellow]Total Time:[white] %.2fms", totalTime))
	return result.String()
}

// Streaming callback functions
func (app *Application) onEntryAdded(entry har.HAREntry, index int) {
	// Batch updates to avoid rubberbanding during scrolling
	entryCount := app.streamingLoader.GetEntryCount()
	if entryCount-app.lastUpdateCount >= app.batchUpdateSize {
		app.lastUpdateCount = entryCount
		app.app.QueueUpdateDraw(func() {
			// Preserve current selection
			currentIndex := app.requests.GetCurrentItem()
			app.updateRequestsList()
			// Restore selection if valid
			if currentIndex >= 0 && currentIndex < len(app.filteredEntries) {
				app.requests.SetCurrentItem(currentIndex)
			}
			app.updateBottomBar()
		})
	} else {
		// Just update progress without rebuilding list
		app.app.QueueUpdateDraw(func() {
			app.updateBottomBar()
		})
	}
}

func (app *Application) onLoadingComplete() {
	app.app.QueueUpdateDraw(func() {
		app.isLoading = false
		app.harData = &har.HARFile{
			Log: har.HARLog{
				Version: "1.2",
				Entries: app.streamingLoader.GetEntries(),
			},
		}
		// Preserve current selection during final update
		currentIndex := app.requests.GetCurrentItem()
		app.updateRequestsList()
		if currentIndex >= 0 && currentIndex < len(app.filteredEntries) {
			app.requests.SetCurrentItem(currentIndex)
		}
		app.updateBottomBar()
		app.showStatusMessage("Loading complete!")
	})
}

func (app *Application) onLoadingError(err error) {
	app.app.QueueUpdateDraw(func() {
		app.isLoading = false
		app.showStatusMessage(fmt.Sprintf("Loading error: %v", err))
	})
}

func (app *Application) onLoadingProgress(count int) {
	app.loadingProgress = count
	app.app.QueueUpdateDraw(func() {
		app.updateBottomBar()
	})
}

// saveFilteredHAR saves the currently filtered HAR entries to a new file
func (app *Application) saveFilteredHAR() {
	// Generate descriptive filename based on current filters
	filename := app.filterState.GenerateFilteredFilename(app.filename)
	
	var harData *har.HARFile
	if app.isLoading {
		harData = &har.HARFile{
			Log: har.HARLog{
				Version: "1.2",
				Entries: app.streamingLoader.GetEntries(),
			},
		}
	} else {
		harData = app.harData
	}
	
	// Save the filtered HAR file
	if err := har.SaveFilteredHAR(harData, app.filteredEntries, filename); err != nil {
		app.showStatusMessage(fmt.Sprintf("Error saving filtered HAR: %v", err))
		return
	}
	
	// Show success message with entry count
	entryCount := len(app.filteredEntries)
	var totalCount int
	if app.isLoading {
		totalCount = app.streamingLoader.GetEntryCount()
	} else {
		totalCount = len(app.harData.Log.Entries)
	}
	app.showStatusMessage(fmt.Sprintf("Saved %d/%d entries to %s", entryCount, totalCount, filename))
}