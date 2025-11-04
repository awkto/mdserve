# Markdown Server

**mdserve** is a simple, lightweight webserver for viewing your markdown files in a browser.

## Features

- Automatic directory listing on homepage
- Clean rendering of markdown files with styling
- **Dynamic table of contents** - automatically generated from markdown headings
- Configurable TOC position (left or right sidebar)
- Smooth scrolling navigation to headings
- Support for tables, code blocks, and all standard markdown features
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
4. The table of contents (if headings exist) appears in the sidebar
5. Click on any TOC item to smoothly scroll to that section

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
