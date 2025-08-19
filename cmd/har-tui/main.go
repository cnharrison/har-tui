package main

import (
	"fmt"
	"log"
	"os"

	"github.com/cnharrison/har-tui/internal/har"
	"github.com/cnharrison/har-tui/internal/ui"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: har-tui <file.har>")
		fmt.Println("\n🐱 HAR TUI DELUXE - A sleek terminal interface for HAR files")
		fmt.Println("Press ? for help when running")
		os.Exit(1)
	}

	harFile := os.Args[1]
	
	// Check if we should use streaming mode (for large files or by default)
	useStreaming := true
	
	if useStreaming {
		// Start the TUI application with streaming loader
		app := ui.NewApplicationStreaming(harFile)
		if err := app.Run(); err != nil {
			log.Fatalf("Error running application: %v", err)
		}
	} else {
		// Legacy mode: Load entire HAR file at once
		data, err := har.LoadHARFile(harFile)
		if err != nil {
			log.Fatalf("Error loading HAR file: %v", err)
		}

		// Start the TUI application
		app := ui.NewApplication(data, harFile)
		if err := app.Run(); err != nil {
			log.Fatalf("Error running application: %v", err)
		}
	}
}