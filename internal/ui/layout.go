package ui

import (
	"github.com/rivo/tview"
)

// createLayout builds the main application layout
func (app *Application) createLayout() {
	// Create centered search container
	searchContainer := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).              // Left spacer
		AddItem(app.searchInput, 0, searchInputWidthRatio, false).  // Search input (2x width)
		AddItem(nil, 0, 1, false)               // Right spacer
	
	// Create top panel that can switch between requests list and waterfall view
	app.topPanel = tview.NewPages()
	app.topPanel.AddPage("requests", app.requests, true, true)
	app.topPanel.AddPage("waterfall", app.waterfallView, true, false)
	
	// Create requests panel with filter bar and search
	requestsPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(app.filterBar, 1, 0, false).
		AddItem(searchContainer, searchBoxHeight, 0, false).  // Height for the bordered search box
		AddItem(app.topPanel, 0, 1, true)
	
	// Main layout
	app.layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(app.topBar, 1, 0, false).
		AddItem(requestsPanel, 0, 1, true).
		AddItem(app.tabBar, 1, 0, false).
		AddItem(app.tabs, 0, tabsHeightRatio, false).
		AddItem(app.bottomBar, 1, 0, false)
}