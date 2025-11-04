# Markdown Server

**mdserve** is a simple, lightweight webserver for viewing your markdown files in a browser.

## Features

- Automatic directory listing on homepage
- Clean rendering of markdown files with professional styling
- **Hierarchical table of contents** - automatically generated from markdown headings
  - Collapsible/expandable sections with visual indicators
  - Tree structure with connecting lines showing relationships
  - Expand all / Collapse all buttons for quick navigation
  - Consistent font sizing throughout TOC
- Configurable TOC position (left or right sidebar)
- Resizable TOC sidebar for customizable layout
- Smooth scrolling navigation to headings
- Proper heading size hierarchy (H1-H6) in rendered content
- Support for custom heading IDs with `{#custom-id}` syntax
- Support for tables, code blocks, and all standard markdown features
- Properly indented bullet points and ordered lists
- View raw markdown source with syntax highlighting (toggle view)
- Works with heading IDs that contain numbers, periods, and special characters
- Browse files from any directory
- Mobile responsive design
- No authentication or encryption - just simple markdown viewing

## Setup

### Install Go

```bash
sudo apt install golang -y
```

### Initialize (first time only)

```bash
go mod init markdown_server
go get github.com/gomarkdown/markdown
```

## Usage

### Basic usage (serve current directory)

```bash
go run mdserve.go
```

### Serve a different directory

```bash
go run mdserve.go /path/to/your/docs
# or
go run mdserve.go -dir /path/to/your/docs
```

### Use a custom port

```bash
go run mdserve.go -port 3000
# or combine with directory
go run mdserve.go /path/to/docs -port 3000
```

### Configure table of contents position

```bash
# TOC on the left (default)
go run mdserve.go -toc left

# TOC on the right
go run mdserve.go -toc right

# Combine all options
go run mdserve.go /path/to/docs -port 3000 -toc right
```

### Access the server

1. Open your browser to `http://localhost:8080`
2. The homepage lists all markdown files and directories
3. Click on any file to view it rendered as HTML
4. The table of contents (if headings exist) appears in the sidebar with a hierarchical tree structure
5. Click on any TOC item to smoothly scroll to that section
6. Use the ▶/▼ icons to expand/collapse TOC sections
7. Use the + and − buttons at the top to expand all or collapse all sections
8. Click "Show Source" button to view the raw markdown with syntax highlighting
9. Resize the TOC sidebar by dragging its edge

## Command-line Options

- `<directory>` - Positional argument to specify directory to serve (default: current directory)
- `-dir <path>` - Flag to specify directory to serve
- `-port <number>` - Port to serve on (default: 8080)
- `-toc <position>` - Table of contents position: 'left' or 'right' (default: left)

## Build for deployment

```bash
go build -o mdserve mdserve.go
./mdserve /path/to/docs
```
