package clipboard

import (
	"fmt"
	"os/exec"
	"strings"
)

// CopyToClipboard copies text to the system clipboard
func CopyToClipboard(text string) error {
	// Try different clipboard commands
	commands := [][]string{
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
		{"pbcopy"}, // macOS
		{"clip"},   // Windows
	}
	
	for _, cmdArgs := range commands {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	
	return fmt.Errorf("no clipboard utility found (tried xclip, xsel, pbcopy, clip)")
}