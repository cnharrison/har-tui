package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/cnharrison/har-tui/internal/har"
	"github.com/cnharrison/har-tui/internal/export"
	"github.com/cnharrison/har-tui/pkg/clipboard"
)

// showHelpModal displays the help modal
func (app *Application) showHelpModal() {
	helpText := `[yellow]🐱 HAR TUI DELUXE - Command Help[white]

[yellow]Navigation:[white]
  [cyan]j/k[white]          Move up/down in focused panel
  [cyan]g/G[white]          Go to top/bottom
  [cyan]h/l[white]          Switch tabs left/right (when focused on bottom)
  [cyan]i[white]            Switch focus between requests and detail panels
  [cyan]w[white]            Toggle between requests list and waterfall view
  [cyan]Tab[white]          Switch between tabs in detail panel
  [cyan]Ctrl+D/U[white]     Page down/up in focused detail panel

[yellow]Waterfall View:[white]
  [cyan]w[white]            Toggle detailed timing breakdown (when in waterfall)
  [cyan]+/=[white]          Zoom in (increase chart width)
  [cyan]-/_[white]          Zoom out (decrease chart width)

[yellow]Filtering & Sorting:[white]
  [cyan]/[white]            Open filter dialog (host/path)
  [cyan]h/l[white]          Navigate type filter buttons (when top focused)
  [cyan]Enter[white]        Activate selected type filter (when top focused)
  [cyan]s[white]            Toggle sort by slowest requests
  [cyan]e[white]            Toggle errors-only view (4xx/5xx)
  [cyan]a[white]            Reset all filters and sorting

[yellow]Actions:[white]
  [cyan]b[white]            Save current response body to file
  [cyan]c[white]            Save current request as cURL command
  [cyan]m[white]            Generate markdown summary and copy to clipboard
  [cyan]y[white]            Copy modal - copy various request/response parts
  [cyan]E[white]            Edit request/response content in $EDITOR
  [cyan]R[white]            Replay current request
  [cyan]S[white]            Save filtered HAR entries to new file
  [cyan]q[white]            Quit application`

	// Create help text view
	helpView := tview.NewTextView()
	helpView.SetDynamicColors(true)
	helpView.SetText(helpText)
	helpView.SetTextAlign(tview.AlignLeft)
	helpView.SetBorder(true)
	helpView.SetTitle(" 🆘 Help ")
	helpView.SetTitleAlign(tview.AlignCenter)
	helpView.SetBorderColor(tcell.ColorYellow)
	
	// Create a flex container for centering
	helpContainer := tview.NewFlex().SetDirection(tview.FlexRow)
	helpContainer.AddItem(nil, 0, 1, false) // Top spacer
	helpContainer.AddItem(
		tview.NewFlex().
			AddItem(nil, 0, 1, false).           // Left spacer
			AddItem(helpView, 0, 2, true).       // Help content (wider)
			AddItem(nil, 0, 1, false),           // Right spacer
		0, 2, true)
	helpContainer.AddItem(nil, 0, 1, false) // Bottom spacer

	// Set input capture for the help container
	helpContainer.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' || event.Key() == tcell.KeyEscape || event.Rune() == '?' {
			app.app.SetRoot(app.layout, true)
			return nil
		}
		return event
	})

	app.app.SetFocus(helpContainer)
	app.app.SetRoot(helpContainer, true)
}

// showReplayModal displays the replay confirmation modal
func (app *Application) showReplayModal(entry har.HAREntry) {
	// Create the full modal text with clear Yes/No options
	fullText := fmt.Sprintf(`Replay request to:
[cyan]%s[white]

[yellow]Press Y to proceed, N to cancel[white]`, entry.Request.URL)
	
	replayView := tview.NewTextView()
	replayView.SetDynamicColors(true)
	replayView.SetText(fullText)
	replayView.SetTextAlign(tview.AlignCenter)
	replayView.SetBorder(true)
	replayView.SetTitle(" 🔄 Replay Request ")
	replayView.SetTitleAlign(tview.AlignCenter)
	replayView.SetBorderColor(tcell.ColorPurple)
	
	// Create a centered container with proper sizing
	replayContainer := tview.NewFlex().SetDirection(tview.FlexRow)
	replayContainer.AddItem(nil, 0, 1, false) // Top spacer
	replayContainer.AddItem(
		tview.NewFlex().
			AddItem(nil, 0, 1, false).           // Left spacer
			AddItem(replayView, 0, 2, true).     // Replay content (wider)
			AddItem(nil, 0, 1, false),           // Right spacer
		8, 0, true) // Fixed height for the modal
	replayContainer.AddItem(nil, 0, 1, false) // Bottom spacer

	// Set focus to make input capture work
	app.app.SetFocus(replayContainer)
	
	replayContainer.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'y', 'Y':
			// Immediately return to main app and execute
			app.app.SetRoot(app.layout, true)
			
			go func() {
				curl := export.GenerateCurlCommand(entry)
				cmd := exec.Command("sh", "-c", curl)
				out, err := cmd.CombinedOutput()
				
				app.app.QueueUpdateDraw(func() {
					result := "[green]✓[white] Replay completed!\n\n"
					if err != nil {
						result = "[red]✗[white] Replay failed!\n\n"
						result += fmt.Sprintf("[red]Error:[white] %v\n\n", err)
					}
					result += "[yellow]Output:[white]\n" + string(out)
					
					app.showResultModal(result)
				})
			}()
			return nil
		case 'n', 'N', 'q':
			app.app.SetRoot(app.layout, true)
			return nil
		case '?':
			app.showHelpModal()
			return nil
		}
		
		// Handle key events too
		switch event.Key() {
		case tcell.KeyEscape:
			app.app.SetRoot(app.layout, true)
			return nil
		}
		
		return event
	})

	app.app.SetRoot(replayContainer, true)
}

// showResultModal displays the result of an operation
func (app *Application) showResultModal(result string) {
	resultView := tview.NewTextView()
	resultView.SetDynamicColors(true)
	resultView.SetText(result + "\n\n[dim]Press any key to close[white]")
	resultView.SetTextAlign(tview.AlignLeft)
	resultView.SetBorder(true)
	resultView.SetTitle(" 📄 Result ")
	resultView.SetTitleAlign(tview.AlignCenter)
	resultView.SetBorderColor(tcell.ColorGreen)
	
	// Create a centered container
	resultContainer := tview.NewFlex().SetDirection(tview.FlexRow)
	resultContainer.AddItem(nil, 0, 1, false) // Top spacer
	resultContainer.AddItem(
		tview.NewFlex().
			AddItem(nil, 0, 1, false).           // Left spacer
			AddItem(resultView, 0, 3, true).     // Result content (larger)
			AddItem(nil, 0, 1, false),           // Right spacer
		0, 2, true)
	resultContainer.AddItem(nil, 0, 1, false) // Bottom spacer

	resultContainer.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		app.app.SetRoot(app.layout, true)
		return nil
	})

	app.app.SetFocus(resultContainer)
	app.app.SetRoot(resultContainer, true)
}

// showCopyModal displays the copy options modal
func (app *Application) showCopyModal(entry har.HAREntry) {
	// Check availability
	hasRequestBody := entry.Request.PostData != nil && entry.Request.PostData.Text != ""
	hasResponseBody := entry.Response.Content.Text != ""
	
	// Build copy options text
	var copyText strings.Builder
	copyText.WriteString("Select content to copy to clipboard:\n\n")
	copyText.WriteString("[yellow]1[white] - Request URL\n")
	copyText.WriteString("[yellow]2[white] - Request Headers (JSON)\n")
	
	// Request Body - strikethrough if not available
	if hasRequestBody {
		copyText.WriteString("[yellow]3[white] - Request Body\n")
	} else {
		copyText.WriteString("[dim]3 - Request Body (empty)[-]\n")
	}
	
	copyText.WriteString("[yellow]4[white] - Response Headers (JSON)\n")
	
	// Response Body - strikethrough if not available
	if hasResponseBody {
		copyText.WriteString("[yellow]5[white] - Response Body\n")
	} else {
		copyText.WriteString("[dim]5 - Response Body (empty)[-]\n")
	}
	
	copyText.WriteString("[yellow]6[white] - Timing Information\n")
	copyText.WriteString("[yellow]7[white] - Full Request Summary\n")
	copyText.WriteString("[yellow]8[white] - Full Response Summary\n")
	copyText.WriteString("[yellow]9[white] - cURL Command\n")
	copyText.WriteString("[yellow]0[white] - Raw JSON (Complete Entry)\n")
	copyText.WriteString("[yellow]m[white] - Markdown Summary\n")
	copyText.WriteString("[yellow]q[white] - Cancel")
	
	copyView := tview.NewTextView()
	copyView.SetDynamicColors(true)
	copyView.SetText(copyText.String())
	copyView.SetTextAlign(tview.AlignCenter)
	copyView.SetBorder(true)
	copyView.SetTitle(" 📋 Copy to Clipboard ")
	copyView.SetTitleAlign(tview.AlignCenter)
	copyView.SetBorderColor(tcell.ColorTeal)
	
	// Create a centered container
	copyContainer := tview.NewFlex().SetDirection(tview.FlexRow)
	copyContainer.AddItem(nil, 0, 1, false) // Top spacer
	copyContainer.AddItem(
		tview.NewFlex().
			AddItem(nil, 0, 1, false).           // Left spacer
			AddItem(copyView, 0, 1, true).       // Copy content
			AddItem(nil, 0, 1, false),           // Right spacer
		14, 0, true) // Fixed height
	copyContainer.AddItem(nil, 0, 1, false) // Bottom spacer

	copyContainer.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		var content string
		var description string
		
		// Handle escape key first
		if event.Key() == tcell.KeyEscape {
			app.app.SetRoot(app.layout, true)
			return nil
		}
		
		switch event.Rune() {
		case '1':
			content = entry.Request.URL
			description = "Request URL copied"
		case '2':
			headersJSON, _ := json.MarshalIndent(entry.Request.Headers, "", "  ")
			content = string(headersJSON)
			description = "Request headers copied"
		case '3':
			if !hasRequestBody {
				app.showStatusMessage("Request body is empty - nothing to copy")
				app.app.SetRoot(app.layout, true)
				return nil
			}
			content = entry.Request.PostData.Text
			description = "Request body copied"
		case '4':
			headersJSON, _ := json.MarshalIndent(entry.Response.Headers, "", "  ")
			content = string(headersJSON)
			description = "Response headers copied"
		case '5':
			if !hasResponseBody {
				app.showStatusMessage("Response body is empty - nothing to copy")
				app.app.SetRoot(app.layout, true)
				return nil
			}
			bodyText := har.DecodeBase64(entry.Response.Content.Text, entry.Response.Content.Encoding)
			content = bodyText
			description = "Response body copied"
		case '6':
			content = fmt.Sprintf("Timing Breakdown:\nDNS: %.2fms\nConnect: %.2fms\nSSL: %.2fms\nSend: %.2fms\nWait: %.2fms\nReceive: %.2fms\nTotal: %.2fms",
				entry.Timings.DNS, entry.Timings.Connect, entry.Timings.SSL,
				entry.Timings.Send, entry.Timings.Wait, entry.Timings.Receive, entry.Time)
			description = "Timing information copied"
		case '7':
			var reqSummary strings.Builder
			reqSummary.WriteString(fmt.Sprintf("Request Summary:\nMethod: %s\nURL: %s\nHTTP Version: %s\n\n", 
				entry.Request.Method, entry.Request.URL, entry.Request.HTTPVersion))
			reqSummary.WriteString("Headers:\n")
			for _, header := range entry.Request.Headers {
				reqSummary.WriteString(fmt.Sprintf("  %s: %s\n", header.Name, header.Value))
			}
			if entry.Request.PostData != nil && entry.Request.PostData.Text != "" {
				reqSummary.WriteString(fmt.Sprintf("\nBody:\n%s", entry.Request.PostData.Text))
			}
			content = reqSummary.String()
			description = "Request summary copied"
		case '8':
			var respSummary strings.Builder
			respSummary.WriteString(fmt.Sprintf("Response Summary:\nStatus: %d %s\nHTTP Version: %s\nContent Type: %s\nSize: %d bytes\n\n",
				entry.Response.Status, entry.Response.StatusText, entry.Response.HTTPVersion,
				entry.Response.Content.MimeType, entry.Response.Content.Size))
			respSummary.WriteString("Headers:\n")
			for _, header := range entry.Response.Headers {
				respSummary.WriteString(fmt.Sprintf("  %s: %s\n", header.Name, header.Value))
			}
			bodyText := har.DecodeBase64(entry.Response.Content.Text, entry.Response.Content.Encoding)
			if bodyText != "" {
				respSummary.WriteString(fmt.Sprintf("\nBody:\n%s", bodyText))
			}
			content = respSummary.String()
			description = "Response summary copied"
		case '9':
			content = export.GenerateCurlCommand(entry)
			description = "cURL command copied"
		case '0':
			rawJSON, _ := json.MarshalIndent(entry, "", "  ")
			content = string(rawJSON)
			description = "Raw JSON entry copied"
		case 'm':
			content = export.GenerateMarkdownSummary(entry)
			description = "Markdown summary copied"
		case 'q':
			app.app.SetRoot(app.layout, true)
			return nil
		default:
			return event
		}
		
		if content != "" {
			if err := clipboard.CopyToClipboard(content); err == nil {
				app.showStatusMessage(description + " to clipboard!")
			} else {
				app.showStatusMessage(fmt.Sprintf("Clipboard error: %v", err))
			}
		}
		
		app.app.SetRoot(app.layout, true)
		return nil
	})

	app.app.SetFocus(copyContainer)
	app.app.SetRoot(copyContainer, true)
}

// showEditorModal displays the editor selection modal
func (app *Application) showEditorModal(currentIndex int) {
	entryIdx := app.filteredEntries[currentIndex]
	entry := app.harData.Log.Entries[entryIdx]
	
	// Check availability
	hasRequestBody := entry.Request.PostData != nil && entry.Request.PostData.Text != ""
	hasResponseBody := entry.Response.Content.Text != ""
	
	// Build conditional editor options
	var editorText strings.Builder
	editorText.WriteString("Select content to edit in $EDITOR:\n\n")
	editorText.WriteString("[yellow]1[white] - Request Headers\n")
	
	// Request Body - strikethrough if not available
	if hasRequestBody {
		editorText.WriteString("[yellow]2[white] - Request Body\n")
	} else {
		editorText.WriteString("[dim]2 - Request Body (empty)[-]\n")
	}
	
	editorText.WriteString("[yellow]3[white] - Response Headers\n")
	
	// Response Body - strikethrough if not available
	if hasResponseBody {
		editorText.WriteString("[yellow]4[white] - Response Body\n")
	} else {
		editorText.WriteString("[dim]4 - Response Body (empty)[-]\n")
	}
	
	editorText.WriteString("[yellow]q[white] - Cancel")
	
	editorView := tview.NewTextView()
	editorView.SetDynamicColors(true)
	editorView.SetText(editorText.String())
	editorView.SetTextAlign(tview.AlignCenter)
	editorView.SetBorder(true)
	editorView.SetTitle(" ✏️  Edit Content ")
	editorView.SetTitleAlign(tview.AlignCenter)
	editorView.SetBorderColor(tcell.ColorGreen)
	
	// Create a centered container
	editorContainer := tview.NewFlex().SetDirection(tview.FlexRow)
	editorContainer.AddItem(nil, 0, 1, false) // Top spacer
	editorContainer.AddItem(
		tview.NewFlex().
			AddItem(nil, 0, 1, false).           // Left spacer
			AddItem(editorView, 0, 1, true).     // Editor content
			AddItem(nil, 0, 1, false),           // Right spacer
		10, 0, true) // Fixed height
	editorContainer.AddItem(nil, 0, 1, false) // Bottom spacer

	editorContainer.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case '1':
			// Edit request headers
			headersJSON, _ := json.MarshalIndent(entry.Request.Headers, "", "  ")
			if _, err := app.openInEditor(string(headersJSON), "json"); err == nil {
				app.showStatusMessage("Request headers edited (note: changes not saved to HAR)")
				app.app.SetRoot(app.layout, true)
			} else {
				app.showStatusMessage(fmt.Sprintf("Editor error: %v", err))
				app.app.SetRoot(app.layout, true)
			}
			return nil
		case '2':
			if !hasRequestBody {
				app.showStatusMessage("Request body is empty - nothing to edit")
				app.app.SetRoot(app.layout, true)
				return nil
			}
			
			content := entry.Request.PostData.Text
			extension := app.getExtensionFromMimeType(entry.Request.PostData.MimeType)
			
			if _, err := app.openInEditor(content, extension); err == nil {
				app.showStatusMessage("Request body edited (note: changes not saved to HAR)")
				app.app.SetRoot(app.layout, true)
			} else {
				app.showStatusMessage(fmt.Sprintf("Editor error: %v", err))
				app.app.SetRoot(app.layout, true)
			}
			return nil
		case '3':
			// Edit response headers
			headersJSON, _ := json.MarshalIndent(entry.Response.Headers, "", "  ")
			if _, err := app.openInEditor(string(headersJSON), "json"); err == nil {
				app.showStatusMessage("Response headers edited (note: changes not saved to HAR)")
				app.app.SetRoot(app.layout, true)
			} else {
				app.showStatusMessage(fmt.Sprintf("Editor error: %v", err))
				app.app.SetRoot(app.layout, true)
			}
			return nil
		case '4':
			if !hasResponseBody {
				app.showStatusMessage("Response body is empty - nothing to edit")
				app.app.SetRoot(app.layout, true)
				return nil
			}
			
			content := har.DecodeBase64(entry.Response.Content.Text, entry.Response.Content.Encoding)
			extension := app.getExtensionFromMimeType(entry.Response.Content.MimeType)
			
			if _, err := app.openInEditor(content, extension); err == nil {
				app.showStatusMessage("Response body edited (note: changes not saved to HAR)")
				app.app.SetRoot(app.layout, true)
			} else {
				app.showStatusMessage(fmt.Sprintf("Editor error: %v", err))
				app.app.SetRoot(app.layout, true)
			}
			return nil
		case 'q':
			app.app.SetRoot(app.layout, true)
			return nil
		}
		
		switch event.Key() {
		case tcell.KeyEscape:
			app.app.SetRoot(app.layout, true)
			return nil
		}
		
		return event
	})

	app.app.SetFocus(editorContainer)
	app.app.SetRoot(editorContainer, true)
}

// openInEditor opens content in the system editor
func (app *Application) openInEditor(content, extension string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi" // fallback
	}
	
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "har-tui-edit-*."+extension)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())
	
	// Write content to temp file
	if _, err := tmpFile.WriteString(content); err != nil {
		return "", err
	}
	tmpFile.Close()
	
	// Open in editor
	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		return "", err
	}
	
	// Read back the edited content
	editedContent, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", err
	}
	
	return string(editedContent), nil
}

// getExtensionFromMimeType returns file extension based on MIME type
func (app *Application) getExtensionFromMimeType(mimeType string) string {
	mimeType = strings.ToLower(mimeType)
	switch {
	case strings.Contains(mimeType, "json"):
		return "json"
	case strings.Contains(mimeType, "html"):
		return "html"
	case strings.Contains(mimeType, "css"):
		return "css"
	case strings.Contains(mimeType, "javascript"):
		return "js"
	case strings.Contains(mimeType, "xml"):
		return "xml"
	default:
		return "txt"
	}
}