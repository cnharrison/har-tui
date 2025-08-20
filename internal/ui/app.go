package ui

import (
	"time"

	"github.com/rivo/tview"
	"github.com/cnharrison/har-tui/internal/har"
	"github.com/cnharrison/har-tui/internal/filter"
	"github.com/cnharrison/har-tui/internal/format"
)

const (
	// Animation and timing constants
	animationIntervalMs = 500
	statusMessageDurationSec = 5
	animationCycleFrames = 4
	pulseCycleFrames = 2
	
	// Layout constants
	searchInputWidthRatio = 2
	searchBoxHeight = 3
	tabsHeightRatio = 2
	maxPathDisplayLength = 50
	pathTruncateOffset = 3
	
	// HTTP status code thresholds
	statusCodeSuccess = 200
	statusCodeRedirect = 300
	statusCodeClientError = 400
	
	// Batch processing
	defaultBatchUpdateSize = 100
	
	// Visual display constants
	timingBarMaxWidth = 40
	minBarWidth = 1
	percentageMultiplier = 100
	
	// HAR version
	harVersion = "1.2"
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
	
	// Side-by-side layout state
	sideBySideViews [2]*tview.TextView // [0] = left pane, [1] = right pane
	isSideBySide    bool
	
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
	
	// Body tab side-by-side flex container (when isSideBySide is true)
	bodyFlexContainer *tview.Flex
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
		batchUpdateSize: defaultBatchUpdateSize,
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

// showStatusMessage shows a temporary status message
func (app *Application) showStatusMessage(msg string) {
	app.confirmationMessage = msg
	app.confirmationEnd = time.Now().Add(statusMessageDurationSec * time.Second)
}