package ui

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/rivo/tview"
	"github.com/cnharrison/har-tui/internal/har"
)

// getCurrentView returns the currently active text view for scrolling
func (app *Application) getCurrentView() *tview.TextView {
	views := []*tview.TextView{app.requestView, app.responseView, app.bodyView, app.cookiesView, app.timingsView, app.rawView}
	if app.currentTab >= 0 && app.currentTab < len(views) {
		return views[app.currentTab]
	}
	return app.requestView
}

// getBlinkingArrows returns blinking arrow characters
func (app *Application) getBlinkingArrows() string {
	if app.animationFrame%animationCycleFrames < pulseCycleFrames {
		return "►"
	}
	return " "
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
	
	maxWidth := timingBarMaxWidth
	for _, phase := range phases {
		if phase.value > 0 {
			percentage := (phase.value / totalTime) * percentageMultiplier
			barWidth := int((phase.value / totalTime) * float64(maxWidth))
			if barWidth < minBarWidth && phase.value > 0 {
				barWidth = minBarWidth
			}
			
			bar := strings.Repeat("█", barWidth) + strings.Repeat("░", maxWidth-barWidth)
			result.WriteString(fmt.Sprintf("%-12s [%s]%s[white] %.2fms (%.1f%%)\n", 
				phase.name, phase.color, bar, phase.value, percentage))
		}
	}
	
	result.WriteString(fmt.Sprintf("\n[yellow]Total Time:[white] %.2fms", totalTime))
	return result.String()
}