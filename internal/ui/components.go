package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

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