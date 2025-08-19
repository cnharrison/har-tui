package ui

import (
	"fmt"

	"github.com/cnharrison/har-tui/internal/har"
)

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
				Version: harVersion,
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