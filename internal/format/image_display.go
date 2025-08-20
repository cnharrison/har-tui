package format

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/blacktop/go-termimg"
	"github.com/muesli/termenv"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/webp"
	"golang.org/x/term"
)

// TerminalCapability represents different image display capabilities
type TerminalCapability int

const (
	CapabilityNone TerminalCapability = iota
	CapabilityBlocks   // ANSI block characters
	CapabilityKitty    // Kitty graphics protocol
	CapabilityITerm2   // iTerm2 inline images
	CapabilitySixel    // Sixel protocol
)

// ImageDisplayer handles displaying images in various terminal formats
type ImageDisplayer struct {
	capability TerminalCapability
	colorProfile termenv.Profile
	maxWidth   int
	maxHeight  int
}

// NewImageDisplayer creates a new image displayer with auto-detected capabilities
func NewImageDisplayer() *ImageDisplayer {
	// Try to get actual terminal dimensions
	width, height := getTerminalSize()
	if width == 0 {
		width = 80  // Fallback
	}
	if height == 0 {
		height = 24 // Fallback
	}
	
	// Reserve some space for UI elements
	maxWidth := width - 4   // Leave margin for borders
	maxHeight := height / 2 // Use half the screen height for images
	
	// Set reasonable minimums
	if maxWidth < 40 {
		maxWidth = 40
	}
	if maxHeight < 10 {
		maxHeight = 10
	}
	
	displayer := &ImageDisplayer{
		maxWidth:  maxWidth,
		maxHeight: maxHeight,
	}
	
	displayer.detectCapabilities()
	return displayer
}

// detectCapabilities automatically detects the best available display method
//
// Terminal Support Matrix:
//   High Quality (Native Protocols):
//     - iTerm2: iTerm2 Inline Images protocol
//     - Kitty: Kitty Graphics Protocol  
//     - WezTerm: Multiple protocols (Kitty > Sixel > iTerm2)
//     - Ghostty: Sixel + Kitty protocols
//
//   Good Quality (Sixel Protocol):
//     - Windows Terminal v1.22+: Sixel (may need to be enabled)
//     - GNOME Terminal (recent): Sixel support
//     - Konsole (KDE): Sixel support
//     - xterm: Sixel (if compiled with --enable-sixel-graphics)
//     - Foot (Wayland): Sixel support
//     - Rio Terminal: Sixel support
//     - Alacritty: Experimental Sixel support
//
//   Fallback (Unicode Block Characters):
//     - macOS Terminal.app: No native image support
//     - Hyper: No native image support
//     - st (suckless): No native image support
//     - Any terminal with halfblocks support
//
//   Special Cases:
//     - VS Code Terminal: Sixel passthrough to host terminal
//     - Tmux: Passthrough to underlying terminal (with proper config)
//     - Terminology: Custom image support (handled by termimg)
//
func (id *ImageDisplayer) detectCapabilities() {
	id.colorProfile = termenv.EnvColorProfile()
	
	// Use termimg's detection as primary method
	protocol := termimg.DetectProtocol()
	
	// If termimg detected a high-quality protocol, use it
	switch protocol {
	case termimg.Kitty:
		id.capability = CapabilityKitty
		return
	case termimg.ITerm2:
		id.capability = CapabilityITerm2
		return
	case termimg.Sixel:
		id.capability = CapabilitySixel
		return
	}
	
	// If termimg only detected halfblocks, try enhanced detection for known terminals
	id.capability = id.detectEnhancedCapabilities()
}

// detectEnhancedCapabilities provides enhanced detection for terminals that might not be properly detected
func (id *ImageDisplayer) detectEnhancedCapabilities() TerminalCapability {
	termProgram := strings.ToLower(os.Getenv("TERM_PROGRAM"))
	term := strings.ToLower(os.Getenv("TERM"))
	colorTerm := strings.ToLower(os.Getenv("COLORTERM"))
	wtSession := os.Getenv("WT_SESSION")
	
	// Check for specific terminal programs that support image protocols
	switch {
	case termProgram == "iterm.app":
		// iTerm2 always supports inline images
		return CapabilityITerm2
		
	case termProgram == "kitty":
		// Kitty always supports its graphics protocol
		return CapabilityKitty
		
	case strings.Contains(term, "kitty"):
		// Kitty variant
		return CapabilityKitty
		
	case wtSession != "":
		// Windows Terminal - try Sixel first, fallback to blocks
		// Windows Terminal v1.22+ should support Sixel, but detection may be conservative
		if termimg.SixelSupported() {
			return CapabilitySixel
		}
		// Note: Could add optimistic Sixel attempt here in the future
		// For now, use reliable halfblocks which work universally
		return CapabilityBlocks
		
	case termProgram == "vscode":
		// VS Code terminal - try Sixel passthrough
		if termimg.SixelSupported() {
			return CapabilitySixel
		}
		return CapabilityBlocks
		
	case termProgram == "wezterm":
		// WezTerm supports multiple protocols
		if termimg.KittySupported() {
			return CapabilityKitty
		}
		if termimg.SixelSupported() {
			return CapabilitySixel
		}
		if termimg.ITerm2Supported() {
			return CapabilityITerm2
		}
		return CapabilityBlocks
		
	case strings.Contains(term, "xterm") && (colorTerm == "truecolor" || colorTerm == "24bit"):
		// Modern xterm with true color - likely supports Sixel if compiled correctly
		if termimg.SixelSupported() {
			return CapabilitySixel
		}
		return CapabilityBlocks
		
	case strings.Contains(term, "alacritty"):
		// Alacritty - Sixel support is experimental
		if termimg.SixelSupported() {
			return CapabilitySixel
		}
		return CapabilityBlocks
		
	case termProgram == "gnome-terminal" || strings.Contains(term, "gnome"):
		// GNOME Terminal - newer versions support Sixel
		if termimg.SixelSupported() {
			return CapabilitySixel
		}
		return CapabilityBlocks
		
	case termProgram == "konsole" || strings.Contains(term, "konsole"):
		// Konsole (KDE) - supports Sixel
		if termimg.SixelSupported() {
			return CapabilitySixel
		}
		return CapabilityBlocks
		
	case strings.Contains(term, "foot"):
		// Foot terminal (Wayland) - supports Sixel
		if termimg.SixelSupported() {
			return CapabilitySixel
		}
		return CapabilityBlocks
		
	case strings.Contains(term, "rio"):
		// Rio terminal - supports Sixel
		if termimg.SixelSupported() {
			return CapabilitySixel
		}
		return CapabilityBlocks
		
	default:
		// For unknown terminals, use the safest fallback
		return CapabilityBlocks
	}
}

// SetMaxSize sets the maximum display size for images
func (id *ImageDisplayer) SetMaxSize(width, height int) {
	id.maxWidth = width
	id.maxHeight = height
}

// GetCapability returns the detected terminal capability
func (id *ImageDisplayer) GetCapability() TerminalCapability {
	return id.capability
}

// GetCapabilityString returns a human-readable capability description
func (id *ImageDisplayer) GetCapabilityString() string {
	switch id.capability {
	case CapabilityKitty:
		return "Kitty Graphics Protocol"
	case CapabilityITerm2:
		return "iTerm2 Inline Images"
	case CapabilitySixel:
		return "Sixel Protocol"
	case CapabilityBlocks:
		return "ANSI Block Characters"
	case CapabilityNone:
		return "No Image Support"
	default:
		return "Unknown"
	}
}

// DisplayImage attempts to display an image using the best available method
func (id *ImageDisplayer) DisplayImage(imageData []byte, mimeType string) (string, error) {
	// Check memory constraints first
	maxImageSize := 50 * 1024 * 1024 // 50MB limit
	if len(imageData) > maxImageSize {
		return "", fmt.Errorf("image too large (%d bytes, max %d bytes)", len(imageData), maxImageSize)
	}
	
	// Decode the image
	img, format, err := id.decodeImage(imageData, mimeType)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}
	
	// Check image dimensions for sanity
	bounds := img.Bounds()
	maxPixels := 10000 * 10000 // 100MP limit
	totalPixels := bounds.Dx() * bounds.Dy()
	if totalPixels > maxPixels {
		return "", fmt.Errorf("image too large (%dx%d = %d pixels, max %d pixels)", 
			bounds.Dx(), bounds.Dy(), totalPixels, maxPixels)
	}
	
	// Create info header
	var result strings.Builder
	result.WriteString(fmt.Sprintf("[yellow]Image Preview[white] ([cyan]%s[white], [cyan]%dx%d[white], [magenta]%s[white])\n\n", 
		format, bounds.Dx(), bounds.Dy(), id.GetCapabilityString()))
	
	// Try to display the image using the best available method
	imageOutput, err := id.renderImage(img)
	if err != nil {
		// If rendering fails, fall back to basic info
		result.WriteString(fmt.Sprintf("[dim]Image display failed: %v[white]\n", err))
		result.WriteString(fmt.Sprintf("[dim]Falling back to basic image info[white]\n"))
		return result.String(), nil
	}
	
	result.WriteString(imageOutput)
	return result.String(), nil
}

// decodeImage decodes image data based on MIME type or format detection
func (id *ImageDisplayer) decodeImage(data []byte, mimeType string) (image.Image, string, error) {
	reader := bytes.NewReader(data)
	
	// Try format-specific decoders first based on MIME type
	mimeType = strings.ToLower(mimeType)
	switch {
	case strings.Contains(mimeType, "png"):
		img, err := png.Decode(reader)
		return img, "PNG", err
	case strings.Contains(mimeType, "jpeg") || strings.Contains(mimeType, "jpg"):
		img, err := jpeg.Decode(reader)
		return img, "JPEG", err
	case strings.Contains(mimeType, "gif"):
		img, err := gif.Decode(reader)
		return img, "GIF", err
	case strings.Contains(mimeType, "webp"):
		img, err := webp.Decode(reader)
		return img, "WebP", err
	}
	
	// Fall back to generic image decoding
	reader.Seek(0, 0) // Reset reader
	img, format, err := image.Decode(reader)
	if err != nil {
		return nil, "", err
	}
	
	return img, strings.ToUpper(format), nil
}

// renderImage renders the image using the detected terminal capability
func (id *ImageDisplayer) renderImage(img image.Image) (string, error) {
	switch id.capability {
	case CapabilityKitty, CapabilityITerm2:
		return id.renderWithTermimg(img)
	case CapabilitySixel:
		return id.renderWithTermimg(img) // go-termimg supports sixel too
	case CapabilityBlocks:
		return id.renderWithPixterm(img)
	default:
		return "", fmt.Errorf("no supported rendering method available")
	}
}

// renderWithTermimg uses go-termimg for high-quality image display
func (id *ImageDisplayer) renderWithTermimg(img image.Image) (string, error) {
	// Use termimg to render the image
	termImgObj := termimg.New(img)
	
	// Resize if needed
	bounds := img.Bounds()
	if bounds.Dx() > id.maxWidth || bounds.Dy() > id.maxHeight {
		// Calculate aspect-preserving dimensions
		scale := min(float64(id.maxWidth)/float64(bounds.Dx()), 
				   float64(id.maxHeight)/float64(bounds.Dy()))
		newWidth := int(float64(bounds.Dx()) * scale)
		newHeight := int(float64(bounds.Dy()) * scale)
		termImgObj = termImgObj.Width(newWidth).Height(newHeight)
	}
	
	// Render to string
	rendered, err := termImgObj.Render()
	if err != nil {
		return "", fmt.Errorf("failed to render image: %w", err)
	}
	
	// Convert ANSI escape sequences to tview format
	converted := id.convertImageANSIToTview(rendered)
	
	return converted, nil
}

// renderWithPixterm uses termimg's halfblocks for ANSI block character rendering  
func (id *ImageDisplayer) renderWithPixterm(img image.Image) (string, error) {
	// Use termimg's halfblocks renderer
	termImgObj := termimg.New(img)
	
	// Resize if needed
	bounds := img.Bounds()
	if bounds.Dx() > id.maxWidth || bounds.Dy() > id.maxHeight*2 { // *2 because halfblocks are 2 pixels high
		// Calculate aspect-preserving dimensions
		scale := min(float64(id.maxWidth)/float64(bounds.Dx()), 
				   float64(id.maxHeight*2)/float64(bounds.Dy()))
		newWidth := int(float64(bounds.Dx()) * scale)
		newHeight := int(float64(bounds.Dy()) * scale)
		termImgObj = termImgObj.Width(newWidth).Height(newHeight)
	}
	
	// Force halfblocks protocol
	termImgObj = termImgObj.Protocol(termimg.Halfblocks)
	
	// Render to string
	rendered, err := termImgObj.Render()
	if err != nil {
		return "", fmt.Errorf("failed to render halfblocks: %w", err)
	}
	
	// Convert ANSI escape sequences to tview format
	converted := id.convertImageANSIToTview(rendered)
	
	return converted, nil
}

// RenderSVGAsImage converts SVG to a raster image and renders it
func (id *ImageDisplayer) RenderSVGAsImage(svgContent string) (string, error) {
	// Parse the SVG
	icon, err := oksvg.ReadIconStream(strings.NewReader(svgContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse SVG: %w", err)
	}
	
	// Set up the canvas dimensions
	width := icon.ViewBox.W
	height := icon.ViewBox.H
	
	// Scale if too large
	if width > float64(id.maxWidth) || height > float64(id.maxHeight) {
		scale := min(float64(id.maxWidth)/width, float64(id.maxHeight)/height)
		width *= scale
		height *= scale
	}
	
	// Create a raster image
	w, h := int(width), int(height)
	icon.SetTarget(0, 0, float64(w), float64(h))
	
	// Render to an image
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	scanner := rasterx.NewScannerGV(w, h, rgba, rgba.Bounds())
	raster := rasterx.NewDasher(w, h, scanner)
	
	icon.Draw(raster, 1.0)
	
	// Now render the image using termimg
	termImgObj := termimg.New(rgba)
	rendered, err := termImgObj.Render()
	if err != nil {
		return "", fmt.Errorf("failed to render SVG image: %w", err)
	}
	
	// Convert ANSI escape sequences to tview format
	converted := id.convertImageANSIToTview(rendered)
	
	return converted, nil
}

// getTerminalSize tries to get the current terminal dimensions
func getTerminalSize() (int, int) {
	// Try to get terminal size from stdin
	if term.IsTerminal(int(os.Stdin.Fd())) {
		width, height, err := term.GetSize(int(os.Stdin.Fd()))
		if err == nil {
			return width, height
		}
	}
	
	// Try stderr as fallback
	if term.IsTerminal(int(os.Stderr.Fd())) {
		width, height, err := term.GetSize(int(os.Stderr.Fd()))
		if err == nil {
			return width, height
		}
	}
	
	return 0, 0 // Couldn't determine size
}

// convertImageANSIToTview converts ANSI escape sequences from termimg to tview color format
func (id *ImageDisplayer) convertImageANSIToTview(content string) string {
	// termimg outputs true-color ANSI sequences like: \x1b[38;2;R;G;B;48;2;R;G;Bm
	// We need to convert these to tview's color tag format
	
	// Handle true-color foreground codes: \x1b[38;2;R;G;Bm
	trucolorFgPattern := regexp.MustCompile(`\x1b\[38;2;(\d+);(\d+);(\d+)m`)
	result := trucolorFgPattern.ReplaceAllStringFunc(content, func(match string) string {
		matches := trucolorFgPattern.FindStringSubmatch(match)
		if len(matches) == 4 {
			r, g, b := matches[1], matches[2], matches[3]
			return fmt.Sprintf("[#%02x%02x%02x]", parseColorComponent(r), parseColorComponent(g), parseColorComponent(b))
		}
		return ""
	})
	
	// Handle true-color background codes: \x1b[48;2;R;G;Bm
	trucolorBgPattern := regexp.MustCompile(`\x1b\[48;2;(\d+);(\d+);(\d+)m`)
	result = trucolorBgPattern.ReplaceAllStringFunc(result, func(match string) string {
		matches := trucolorBgPattern.FindStringSubmatch(match)
		if len(matches) == 4 {
			r, g, b := matches[1], matches[2], matches[3]
			return fmt.Sprintf("[:#%02x%02x%02x]", parseColorComponent(r), parseColorComponent(g), parseColorComponent(b))
		}
		return ""
	})
	
	// Handle combined foreground+background codes: \x1b[38;2;R;G;B;48;2;R;G;Bm
	combinedPattern := regexp.MustCompile(`\x1b\[38;2;(\d+);(\d+);(\d+);48;2;(\d+);(\d+);(\d+)m`)
	result = combinedPattern.ReplaceAllStringFunc(result, func(match string) string {
		matches := combinedPattern.FindStringSubmatch(match)
		if len(matches) == 7 {
			fgR, fgG, fgB := matches[1], matches[2], matches[3]
			bgR, bgG, bgB := matches[4], matches[5], matches[6]
			return fmt.Sprintf("[#%02x%02x%02x:#%02x%02x%02x]",
				parseColorComponent(fgR), parseColorComponent(fgG), parseColorComponent(fgB),
				parseColorComponent(bgR), parseColorComponent(bgG), parseColorComponent(bgB))
		}
		return ""
	})
	
	// Handle 256-color foreground codes: \x1b[38;5;Nm
	color256FgPattern := regexp.MustCompile(`\x1b\[38;5;(\d+)m`)
	result = color256FgPattern.ReplaceAllStringFunc(result, func(match string) string {
		matches := color256FgPattern.FindStringSubmatch(match)
		if len(matches) == 2 {
			return map256ColorToTview(matches[1])
		}
		return ""
	})
	
	// Handle 256-color background codes: \x1b[48;5;Nm
	color256BgPattern := regexp.MustCompile(`\x1b\[48;5;(\d+)m`)
	result = color256BgPattern.ReplaceAllStringFunc(result, func(match string) string {
		matches := color256BgPattern.FindStringSubmatch(match)
		if len(matches) == 2 {
			return map256ColorToTviewBg(matches[1])
		}
		return ""
	})
	
	// Handle standard ANSI reset codes
	result = strings.ReplaceAll(result, "\x1b[0m", "[-]")
	result = strings.ReplaceAll(result, "\x1b[39m", "[-]") // Default foreground
	result = strings.ReplaceAll(result, "\x1b[49m", "[-]") // Default background
	
	// Remove any remaining unhandled ANSI escape sequences
	ansiPattern := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	result = ansiPattern.ReplaceAllString(result, "")
	
	return result
}

// parseColorComponent safely parses a color component string to int, clamped to 0-255
func parseColorComponent(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	if n < 0 {
		return 0
	}
	if n > 255 {
		return 255
	}
	return n
}

// map256ColorToTview maps 256-color ANSI codes to tview foreground colors
func map256ColorToTview(colorNum string) string {
	switch colorNum {
	// Standard colors (0-15)
	case "0":  return "[black]"
	case "1":  return "[red]"
	case "2":  return "[green]"
	case "3":  return "[yellow]"
	case "4":  return "[blue]"
	case "5":  return "[magenta]"
	case "6":  return "[cyan]"
	case "7":  return "[white]"
	case "8":  return "[gray]"
	case "9":  return "[red]"
	case "10": return "[green]"
	case "11": return "[yellow]"
	case "12": return "[blue]"
	case "13": return "[magenta]"
	case "14": return "[cyan]"
	case "15": return "[white]"
	default:
		// For other colors, try to map to closest basic color or use hex
		switch {
		case colorNum >= "16" && colorNum <= "51":   // Blues/cyans
			return "[blue]"
		case colorNum >= "52" && colorNum <= "87":   // Greens
			return "[green]"
		case colorNum >= "88" && colorNum <= "123":  // Yellows/oranges
			return "[yellow]"
		case colorNum >= "124" && colorNum <= "159": // Reds
			return "[red]"
		case colorNum >= "160" && colorNum <= "195": // Magentas
			return "[magenta]"
		case colorNum >= "196" && colorNum <= "231": // Cyans/whites
			return "[cyan]"
		case colorNum >= "232" && colorNum <= "255": // Grayscale
			return "[gray]"
		default:
			return "[white]" // Fallback
		}
	}
}

// map256ColorToTviewBg maps 256-color ANSI codes to tview background colors
func map256ColorToTviewBg(colorNum string) string {
	switch colorNum {
	// Standard colors (0-15)
	case "0":  return "[:black]"
	case "1":  return "[:red]"
	case "2":  return "[:green]"
	case "3":  return "[:yellow]"
	case "4":  return "[:blue]"
	case "5":  return "[:magenta]"
	case "6":  return "[:cyan]"
	case "7":  return "[:white]"
	case "8":  return "[:gray]"
	case "9":  return "[:red]"
	case "10": return "[:green]"
	case "11": return "[:yellow]"
	case "12": return "[:blue]"
	case "13": return "[:magenta]"
	case "14": return "[:cyan]"
	case "15": return "[:white]"
	default:
		// For other colors, try to map to closest basic color
		switch {
		case colorNum >= "16" && colorNum <= "51":   // Blues/cyans
			return "[:blue]"
		case colorNum >= "52" && colorNum <= "87":   // Greens
			return "[:green]"
		case colorNum >= "88" && colorNum <= "123":  // Yellows/oranges
			return "[:yellow]"
		case colorNum >= "124" && colorNum <= "159": // Reds
			return "[:red]"
		case colorNum >= "160" && colorNum <= "195": // Magentas
			return "[:magenta]"
		case colorNum >= "196" && colorNum <= "231": // Cyans/whites
			return "[:cyan]"
		case colorNum >= "232" && colorNum <= "255": // Grayscale
			return "[:gray]"
		default:
			return "[:white]" // Fallback
		}
	}
}

// min returns the minimum of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}