package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/cnharrison/har-tui/internal/har"
	"github.com/cnharrison/har-tui/internal/filter"
	"github.com/cnharrison/har-tui/internal/export"
	"github.com/cnharrison/har-tui/pkg/clipboard"
)

// handleInput handles all keyboard input for the application
func (app *Application) handleInput(event *tcell.EventKey) *tcell.EventKey {
	currentIndex := app.requests.GetCurrentItem()
	
	// Handle search input focus specially
	if app.app.GetFocus() == app.searchInput {
		// Handle escape to exit search
		if event.Key() == tcell.KeyEscape {
			app.searchInput.SetTitle(" 🔍 Search ")
			app.searchInput.SetBorderColor(tcell.ColorGreen)
			app.focusOnBottom = false
			app.updateFocusStyles()
			app.app.SetFocus(app.requests)
			return nil
		}
		// Don't block any letter keys when typing in search
		switch event.Key() {
		case tcell.KeyTab, tcell.KeyBacktab:
			return nil // Block tab navigation
		}
		// Let all other keys (including letters for typing) pass through
		return event
	}
	
	// Global navigation
	switch event.Key() {
	case tcell.KeyTab:
		// Tab navigation should always work - switch to next tab
		app.currentTab = (app.currentTab + 1) % 6
		tabNames := []string{"Request", "Response", "Body", "Cookies", "Timings", "Raw"}
		app.tabs.SwitchToPage(tabNames[app.currentTab])
		app.updateTabBar()
		app.updateFocusStyles()
		// Refresh content for the current request when switching tabs
		app.updateTabContent(app.requests.GetCurrentItem())
		return nil
	case tcell.KeyBacktab:
		// Shift+Tab navigation should always work - switch to previous tab
		app.currentTab = (app.currentTab - 1 + 6) % 6
		tabNames := []string{"Request", "Response", "Body", "Cookies", "Timings", "Raw"}
		app.tabs.SwitchToPage(tabNames[app.currentTab])
		app.updateTabBar()
		app.updateFocusStyles()
		// Refresh content for the current request when switching tabs
		app.updateTabContent(app.requests.GetCurrentItem())
		return nil
	case tcell.KeyCtrlD:
		if app.focusOnBottom {
			app.navigateDown(10) // Page down
			return nil
		}
	case tcell.KeyCtrlU:
		if app.focusOnBottom {
			app.navigateUp(10) // Page up
			return nil
		}
	}
	
	switch event.Rune() {
	case '?':
		app.showHelpModal()
	case 'q':
		app.app.Stop()
	case 'i': // 'i' for focus switching between panels
		app.focusOnBottom = !app.focusOnBottom
		if app.focusOnBottom {
			// Focus on bottom panel (tabs)
			app.app.SetFocus(app.getCurrentView())
		} else {
			// Focus on top panel (requests list or waterfall)
			if app.showWaterfall {
				app.app.SetFocus(app.waterfallView)
			} else {
				app.app.SetFocus(app.requests)
			}
		}
		app.updateFocusStyles()
		app.updateBottomBar() // Update context info when focus changes
		return nil
	case 'j':
		if app.focusOnBottom {
			app.navigateDown(1)
		} else if app.showWaterfall {
			// Move down in waterfall selection
			app.waterfallView.MoveDown()
			// Sync requests list selection with waterfall
			app.syncRequestsListFromWaterfall()
		} else if currentIndex < len(app.filteredEntries)-1 {
			app.requests.SetCurrentItem(currentIndex + 1)
		}
		return nil
	case 'k':
		if app.focusOnBottom {
			app.navigateUp(1)
		} else if app.showWaterfall {
			// Move up in waterfall selection
			app.waterfallView.MoveUp()
			// Sync requests list selection with waterfall
			app.syncRequestsListFromWaterfall()
		} else if currentIndex > 0 {
			app.requests.SetCurrentItem(currentIndex - 1)
		}
		return nil
	case 'g':
		if app.focusOnBottom {
			app.navigateToTop()
		} else if app.showWaterfall {
			app.waterfallView.GoToTop()
			app.syncRequestsListFromWaterfall()
		} else {
			app.requests.SetCurrentItem(0)
			app.updateTabContent(0) // Ensure content is updated when jumping to first item
		}
		return nil
	case 'G':
		if app.focusOnBottom {
			app.navigateToBottom()
		} else if app.showWaterfall {
			app.waterfallView.GoToBottom()
			app.syncRequestsListFromWaterfall()
		} else {
			newIndex := len(app.filteredEntries) - 1
			app.requests.SetCurrentItem(newIndex)
			app.updateTabContent(newIndex) // Ensure content is updated when jumping to last item
		}
		return nil
	case 'h':
		if app.focusOnBottom {
			// Navigate tabs when bottom panel is focused
			app.currentTab = (app.currentTab - 1 + 6) % 6
			tabNames := []string{"Request", "Response", "Body", "Cookies", "Timings", "Raw"}
			app.tabs.SwitchToPage(tabNames[app.currentTab])
			app.updateTabBar()
			app.updateFocusStyles()
			// Prevent interference from async updates
			return nil
		} else {
			// Navigate filter buttons when top panel is focused and auto-apply
			typeFilters := filter.GetTypeFilters()
			app.selectedFilterIndex = (app.selectedFilterIndex - 1 + len(typeFilters)) % len(typeFilters)
			app.filterState.SetTypeFilter(typeFilters[app.selectedFilterIndex])
			app.updateRequestsList()
			app.updateBottomBar()
			app.updateFilterBar()
			if typeFilters[app.selectedFilterIndex] == "all" {
				app.showStatusMessage("Showing all request types")
			} else {
				app.showStatusMessage(fmt.Sprintf("Filtering by type: %s", typeFilters[app.selectedFilterIndex]))
			}
		}
	case 'l':
		if app.focusOnBottom {
			// Navigate tabs when bottom panel is focused
			app.currentTab = (app.currentTab + 1) % 6
			tabNames := []string{"Request", "Response", "Body", "Cookies", "Timings", "Raw"}
			app.tabs.SwitchToPage(tabNames[app.currentTab])
			app.updateTabBar()
			app.updateFocusStyles()
			// Prevent interference from async updates
			return nil
		} else {
			// Navigate filter buttons when top panel is focused and auto-apply
			typeFilters := filter.GetTypeFilters()
			app.selectedFilterIndex = (app.selectedFilterIndex + 1) % len(typeFilters)
			app.filterState.SetTypeFilter(typeFilters[app.selectedFilterIndex])
			app.updateRequestsList()
			app.updateBottomBar()
			app.updateFilterBar()
			if typeFilters[app.selectedFilterIndex] == "all" {
				app.showStatusMessage("Showing all request types")
			} else {
				app.showStatusMessage(fmt.Sprintf("Filtering by type: %s", typeFilters[app.selectedFilterIndex]))
			}
		}
	case '/':
		// Focus on search input for inline filtering and clear any content
		app.searchInput.SetText("")
		app.filterState.SetTextFilter("")
		app.updateRequestsList()
		app.updateBottomBar()
		app.app.SetFocus(app.searchInput)
		return nil
	case 's':
		app.filterState.ToggleSortBySlowest()
		app.updateRequestsList()
		app.updateBottomBar()
		if app.filterState.SortBySlowest {
			app.showStatusMessage("Sorted by slowest requests")
		} else {
			app.showStatusMessage("Sorted by original order")
		}
	case 'e':
		app.filterState.ToggleErrorsOnly()
		app.updateRequestsList()
		app.updateBottomBar()
		if app.filterState.ShowErrorsOnly {
			app.showStatusMessage("Showing errors only")
		} else {
			app.showStatusMessage("Showing all requests")
		}
	case 'a':
		app.filterState.Reset()
		app.selectedFilterIndex = 0
		app.searchInput.SetText("") // Clear the search box visually
		app.updateRequestsList()
		app.updateBottomBar()
		app.updateFilterBar()
		app.showStatusMessage("Filters reset")
	case 'b':
		if currentIndex >= 0 && currentIndex < len(app.filteredEntries) {
			entryIdx := app.filteredEntries[currentIndex]
			entry := app.harData.Log.Entries[entryIdx]
			if entry.Response.Content.Text != "" {
				bodyText := entry.Response.Content.Text
				filename := app.generateDescriptiveFilename(entry, ".body.txt")
				if err := os.WriteFile(filename, []byte(bodyText), 0644); err == nil {
					app.showStatusMessage(fmt.Sprintf("Body saved to %s", filename))
				} else {
					app.showStatusMessage(fmt.Sprintf("Error saving body: %v", err))
				}
			}
		}
	case 'c':
		if currentIndex >= 0 && currentIndex < len(app.filteredEntries) {
			entryIdx := app.filteredEntries[currentIndex]
			entry := app.harData.Log.Entries[entryIdx]
			curl := export.GenerateCurlCommand(entry)
			filename := app.generateDescriptiveFilename(entry, ".curl.sh")
			if err := os.WriteFile(filename, []byte(curl), 0755); err == nil {
				app.showStatusMessage(fmt.Sprintf("cURL saved to %s", filename))
			} else {
				app.showStatusMessage(fmt.Sprintf("Error saving cURL: %v", err))
			}
		}
	case 'w':
		// Always toggle between requests list and waterfall view (regardless of focus)
		app.showWaterfall = !app.showWaterfall
		if app.showWaterfall {
			app.topPanel.SwitchToPage("waterfall")
			app.updateWaterfallView()
			app.showStatusMessage("Switched to waterfall view")
		} else {
			app.topPanel.SwitchToPage("requests")
			app.showStatusMessage("Switched to requests list")
		}
		return nil
	case 'd':
		// Toggle detailed timing breakdown (when in waterfall view)
		if app.showWaterfall {
			app.waterfallView.ToggleDetails()
			app.showStatusMessage("Toggled detailed timing breakdown")
		}
		return nil
	case '+', '=':
		// Zoom in waterfall when in waterfall view
		if app.showWaterfall {
			app.waterfallView.ZoomIn()
		}
		return nil
	case '-', '_':
		// Zoom out waterfall when in waterfall view
		if app.showWaterfall {
			app.waterfallView.ZoomOut()
		}
	case 'R':
		if currentIndex >= 0 && currentIndex < len(app.filteredEntries) {
			entryIdx := app.filteredEntries[currentIndex]
			entry := app.harData.Log.Entries[entryIdx]
			app.showReplayModal(entry)
		}
	case 'm': // Generate markdown summary and copy to clipboard
		if currentIndex >= 0 && currentIndex < len(app.filteredEntries) {
			entryIdx := app.filteredEntries[currentIndex]
			entry := app.harData.Log.Entries[entryIdx]
			summary := export.GenerateMarkdownSummary(entry)
			if err := clipboard.CopyToClipboard(summary); err == nil {
				app.showStatusMessage("Markdown summary copied to clipboard!")
			} else {
				app.showStatusMessage(fmt.Sprintf("Clipboard error: %v", err))
			}
		}
	case 'E': // Edit current content in $EDITOR
		if currentIndex >= 0 && currentIndex < len(app.filteredEntries) {
			app.showEditorModal(currentIndex)
		}
	case 'y': // Copy modal (yank)
		if currentIndex >= 0 && currentIndex < len(app.filteredEntries) {
			entryIdx := app.filteredEntries[currentIndex]
			entry := app.harData.Log.Entries[entryIdx]
			app.showCopyModal(entry)
		}
	case 'S': // Save filtered HAR to file
		app.saveFilteredHAR()
	}
	return event
}

// Unified navigation functions that handle both JSON line navigation and regular scrolling

// navigateDown moves down by the specified number of lines/rows
func (app *Application) navigateDown(amount int) {
	if app.isViewingJSON() {
		app.moveJSONLine(amount)
	} else {
		// Regular scroll for non-JSON content
		row, _ := app.getCurrentView().GetScrollOffset()
		app.getCurrentView().ScrollTo(row+amount, 0)
		// Update bottom bar for path tracking
		if app.currentTab == 2 { // Body tab
			app.updateBottomBar()
		}
	}
}

// navigateUp moves up by the specified number of lines/rows
func (app *Application) navigateUp(amount int) {
	if app.isViewingJSON() {
		app.moveJSONLine(-amount)
	} else {
		// Regular scroll for non-JSON content
		row, _ := app.getCurrentView().GetScrollOffset()
		if row >= amount {
			app.getCurrentView().ScrollTo(row-amount, 0)
		} else {
			app.getCurrentView().ScrollTo(0, 0)
		}
		// Update bottom bar for path tracking
		if app.currentTab == 2 { // Body tab
			app.updateBottomBar()
		}
	}
}

// navigateToTop jumps to the beginning of content
func (app *Application) navigateToTop() {
	if app.isViewingJSON() {
		app.currentJSONLine = 1
		app.refreshJSONContent()
		app.updateBottomBar()
	} else {
		app.getCurrentView().ScrollToBeginning()
	}
}

// navigateToBottom jumps to the end of content
func (app *Application) navigateToBottom() {
	if app.isViewingJSON() {
		// Ensure we have the correct total lines by getting the current JSON content
		currentIndex := app.requests.GetCurrentItem()
		if currentIndex >= 0 && currentIndex < len(app.filteredEntries) {
			entryIdx := app.filteredEntries[currentIndex]
			var entries []har.HAREntry
			if app.isLoading {
				entries = app.streamingLoader.GetEntries()
			} else if app.harData != nil {
				entries = app.harData.Log.Entries
			}
			
			if entryIdx < len(entries) {
				entry := entries[entryIdx]
				bodyText := har.DecodeBase64(entry.Response.Content.Text, entry.Response.Content.Encoding)
				if bodyText != "" {
					contentType := app.formatter.DetectContentType(bodyText, entry.Response.Content.MimeType)
					if contentType == "json" {
						// Get the pretty-printed JSON to ensure we have the correct line count
						prettyJSON := app.prettyPrintJSON(bodyText)
						lines := strings.Split(prettyJSON, "\n")
						totalLines := len(lines)
						
						// Now set to the last line
						app.currentJSONLine = totalLines
						app.jsonTotalLines = totalLines
						app.refreshJSONContent()
						app.updateBottomBar()
						return
					}
				}
			}
		}
		// Fallback for non-JSON or error cases
		app.getCurrentView().ScrollToEnd()
	} else {
		app.getCurrentView().ScrollToEnd()
	}
}

// syncRequestsListFromWaterfall syncs the requests list selection to match waterfall selection
func (app *Application) syncRequestsListFromWaterfall() {
	if !app.showWaterfall {
		return
	}
	
	// Get the currently selected entry index from waterfall
	selectedEntryIndex := app.waterfallView.GetSelectedIndex()
	
	// Find the corresponding index in filteredEntries
	for i, entryIdx := range app.filteredEntries {
		if entryIdx == selectedEntryIndex {
			// Update requests list selection without triggering unnecessary updates
			app.requests.SetCurrentItem(i)
			// Update tab content to reflect the new selection
			app.updateTabContent(i)
			break
		}
	}
}

// generateDescriptiveFilename creates descriptive filenames for exported files
func (app *Application) generateDescriptiveFilename(entry har.HAREntry, suffix string) string {
	// This is a placeholder - would need to implement the full logic from har-tui.go
	return fmt.Sprintf("har_entry_%s_%s%s", entry.Request.Method, strings.ReplaceAll(entry.Request.URL, "/", "_"), suffix)
}