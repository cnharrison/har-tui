package ui

import (
	"time"

	"github.com/gdamore/tcell/v2"
)

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
			time.Sleep(animationIntervalMs * time.Millisecond)
			app.animationFrame++
			app.app.QueueUpdateDraw(func() {
				app.updateFocusStyles()
				app.updateBottomBar()
			})
		}
	}()
}