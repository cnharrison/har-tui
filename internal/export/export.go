package export

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/cnharrison/har-tui/internal/har"
)

// GenerateCurlCommand generates a curl command from a HAR entry
func GenerateCurlCommand(entry har.HAREntry) string {
	var cmd strings.Builder
	cmd.WriteString(fmt.Sprintf("curl -X %s '%s'", entry.Request.Method, entry.Request.URL))
	
	for _, header := range entry.Request.Headers {
		if strings.ToLower(header.Name) != "host" {
			cmd.WriteString(fmt.Sprintf(" -H '%s: %s'", header.Name, header.Value))
		}
	}
	
	if entry.Request.PostData != nil && entry.Request.PostData.Text != "" {
		cmd.WriteString(fmt.Sprintf(" -d '%s'", strings.ReplaceAll(entry.Request.PostData.Text, "'", "'\\''")))
	}
	
	return cmd.String()
}

// OpenInEditor opens content in the user's preferred editor
func OpenInEditor(content, extension string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi" // fallback
	}
	
	// Create temporary file
	tmpFile, err := ioutil.TempFile("", "har-tui-edit-*."+extension)
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
	editedContent, err := ioutil.ReadFile(tmpFile.Name())
	if err != nil {
		return "", err
	}
	
	return string(editedContent), nil
}