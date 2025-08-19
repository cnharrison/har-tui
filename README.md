# 🐱 HAR TUI DELUXE
<a href="https://asciinema.org/a/BBe0iZNlqZlX5UMrgem4mGrE5" target="_blank"><img src="https://asciinema.org/a/BBe0iZNlqZlX5UMrgem4mGrE5.svg" /></a>
The world's best TUI HAR viewer. Vibe coded. Vim inspired. Request type filters like on your browser's dev tools. Composable filters. Autodetection, prettification and formatting. Quickly copy any part of any request, or a summary of the whole request, to the clipboard. Quickly output to curl. Replay requests. TUI HAR viewer. You know you want it.

## 🛠 Installation

### Installing Go

If you don't have Go installed:

#### Linux/macOS
```bash
# Download and install Go from https://golang.org/dl/
# Or use a package manager:

# Ubuntu/Debian
sudo apt update && sudo apt install golang-go

# macOS (Homebrew)
brew install go

# Arch Linux
sudo pacman -S go
```

#### Windows
Download the installer from [https://golang.org/dl/](https://golang.org/dl/) or use:
```powershell
# Using Chocolatey
choco install golang

# Using Scoop
scoop install go
```

### Installing HAR TUI

#### Option 1: Install from Source

```bash
# Clone the repository
git clone https://github.com/cnharrison/har-tui.git
cd har-tui

# Build the application
go build -o har-tui cmd/har-tui/main.go

# Make it executable (Linux/macOS)
chmod +x har-tui
```

#### Option 2: Install to System PATH

##### Linux/macOS
```bash
# Build and install to /usr/local/bin
go build -o har-tui cmd/har-tui/main.go
sudo mv har-tui /usr/local/bin/
```

##### Windows (PowerShell)
```powershell
# Build the application
go build -o har-tui.exe cmd/har-tui/main.go

# Move to a directory in your PATH (example: C:\Tools)
mkdir C:\Tools -Force
move har-tui.exe C:\Tools\
```

Then add `C:\Tools` to your PATH environment variable.

#### Option 3: Using Go Install

```bash
go install github.com/cnharrison/har-tui@latest
```

#### Option 4: Using Make

```bash
# Build
make build

# Install to system PATH
make install
```

## ⌨️ Keyboard Shortcuts

### Navigation
| Key | Action |
|-----|--------|
| `j` / `k` | Move up/down in request list |
| `h` / `l` | Navigate filter buttons (top panel) / tabs (bottom panel) |
| `g` / `G` | Go to top/bottom |
| `i` | Switch focus between request list and detail panels |
| `Tab` / `Shift+Tab` | Navigate tabs in detail panel |
| `Ctrl+D` / `Ctrl+U` | Page down/up in focused detail panel |

### Filtering & Search
| Key | Action |
|-----|--------|
| `/` | Open inline search (filter by host/path) |
| `h` / `l` | Navigate type filter buttons when focused on top |
| `s` | Toggle sort by slowest requests |
| `e` | Toggle errors-only view (4xx/5xx) |
| `a` | Reset all filters and sorting |

### Actions
| Key | Action |
|-----|--------|
| `y` | **Copy modal** - Copy various parts to clipboard |
| `b` | Save current response body to file |
| `c` | Save current request as cURL command |
| `m` | Generate markdown summary and copy to clipboard |
| `E` | Edit request/response content in $EDITOR |
| `R` | Replay current request |
| `?` | Show help |
| `q` | Quit application |

### Copy Modal Options (`y`)
| Key | Content |
|-----|---------|
| `1` | Request URL |
| `2` | Request headers (JSON) |
| `3` | Request body |
| `4` | Response headers (JSON) |
| `5` | Response body |
| `6` | Timing information |
| `7` | Full request summary |
| `8` | Full response summary |
| `9` | cURL command |
| `0` | Raw JSON (complete entry) |
| `m` | Markdown summary |

## 📝 License

MIT License - see LICENSE file for details.
