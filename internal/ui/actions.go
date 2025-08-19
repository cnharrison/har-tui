package ui

import (
	"fmt"

	"github.com/cnharrison/har-tui/internal/har"
)

// saveFilteredHAR saves the currently filtered HAR entries to a new file
func (app *Application) saveFilteredHAR() {
	// Generate descriptive filename based on current filters
	filename := app.filterState.GenerateFilteredFilename(app.filename)
	
	var harData *har.HARFile
	if app.isLoading {
		harData = &har.HARFile{
			Log: har.HARLog{
				Version: harVersion,
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