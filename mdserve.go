package main

import (
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gomarkdown/markdown"
)

var baseDir string
var tocPosition string

// FileInfo represents a file or directory for the index
type FileInfo struct {
	Name        string
	Path        string
	IsDirectory bool
}

// Heading represents a markdown heading for TOC
type Heading struct {
	Level int
	Text  string
	ID    string
}

// Clean markdown formatting from text
func cleanMarkdown(text string) string {
	// Remove inline code
	text = regexp.MustCompile("`[^`]+`").ReplaceAllStringFunc(text, func(match string) string {
		return strings.Trim(match, "`")
	})

	// Remove bold/italic markers (**text**, *text*, __text__, _text_)
	text = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`__([^_]+)__`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`_([^_]+)_`).ReplaceAllString(text, "$1")

	// Remove links [text](url) -> text
	text = regexp.MustCompile(`\[([^\]]+)\]\([^\)]+\)`).ReplaceAllString(text, "$1")

	// Remove images ![alt](url) -> alt
	text = regexp.MustCompile(`!\[([^\]]*)\]\([^\)]+\)`).ReplaceAllString(text, "$1")

	// Remove strikethrough ~~text~~
	text = regexp.MustCompile(`~~([^~]+)~~`).ReplaceAllString(text, "$1")

	// Remove HTML tags
	text = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(text, "")

	return strings.TrimSpace(text)
}

// Extract headings from markdown content
func extractHeadings(content []byte) []Heading {
	var headings []Heading
	lines := strings.Split(string(content), "\n")
	headingRegex := regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if matches := headingRegex.FindStringSubmatch(line); matches != nil {
			level := len(matches[1])
			rawText := strings.TrimSpace(matches[2])
			cleanText := cleanMarkdown(rawText)

			// Create a simple ID from the cleaned text
			id := strings.ToLower(cleanText)
			id = regexp.MustCompile(`[^a-z0-9\s-]`).ReplaceAllString(id, "")
			id = regexp.MustCompile(`\s+`).ReplaceAllString(id, "-")

			headings = append(headings, Heading{
				Level: level,
				Text:  cleanText,
				ID:    id,
			})
		}
	}

	return headings
}

// Index handler - lists all markdown files and directories
func indexHandler(w http.ResponseWriter, r *http.Request) {
	var files []FileInfo
	var dirs []FileInfo

	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files and directories
		if strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}

		// Skip the base directory itself
		if relPath == "." {
			return nil
		}

		if info.IsDir() {
			dirs = append(dirs, FileInfo{
				Name:        relPath,
				Path:        relPath,
				IsDirectory: true,
			})
		} else if strings.HasSuffix(info.Name(), ".md") {
			files = append(files, FileInfo{
				Name:        relPath,
				Path:        relPath,
				IsDirectory: false,
			})
		}

		return nil
	})

	if err != nil {
		http.Error(w, fmt.Sprintf("Error listing files: %v", err), http.StatusInternalServerError)
		return
	}

	// Sort directories and files
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Name < dirs[j].Name
	})
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Markdown Server</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            line-height: 1.6;
        }
        h1 {
            border-bottom: 2px solid #333;
            padding-bottom: 10px;
        }
        .section {
            margin: 30px 0;
        }
        .section h2 {
            color: #555;
            font-size: 1.3em;
        }
        ul {
            list-style: none;
            padding: 0;
        }
        li {
            padding: 8px 0;
        }
        a {
            color: #0066cc;
            text-decoration: none;
        }
        a:hover {
            text-decoration: underline;
        }
        .directory {
            font-weight: bold;
        }
        .directory::before {
            content: "📁 ";
        }
        .file::before {
            content: "📄 ";
        }
    </style>
</head>
<body>
    <h1>Markdown Files</h1>

    {{if .Dirs}}
    <div class="section">
        <h2>Directories</h2>
        <ul>
        {{range .Dirs}}
            <li><span class="directory">{{.Name}}/</span></li>
        {{end}}
        </ul>
    </div>
    {{end}}

    <div class="section">
        <h2>Files</h2>
        {{if .Files}}
        <ul>
        {{range .Files}}
            <li><a href="/view/{{.Path}}" class="file">{{.Name}}</a></li>
        {{end}}
        </ul>
        {{else}}
        <p>No markdown files found.</p>
        {{end}}
    </div>
</body>
</html>`

	data := struct {
		Dirs  []FileInfo
		Files []FileInfo
	}{
		Dirs:  dirs,
		Files: files,
	}

	t, err := template.New("index").Parse(tmpl)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
	t.Execute(w, data)
}

// View handler - renders markdown files
func viewHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the file path from URL
	file := r.URL.Path[len("/view/"):]
	if file == "" {
		http.Error(w, "File not specified", http.StatusBadRequest)
		return
	}

	// Construct full path
	fullPath := filepath.Join(baseDir, file)

	// Security check: ensure the resolved path is within baseDir
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}
	if !strings.HasPrefix(absPath, absBaseDir) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Read the file
	content, err := ioutil.ReadFile(fullPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Extract headings for TOC
	headings := extractHeadings(content)

	// Convert markdown to HTML
	htmlContent := markdown.ToHTML(content, nil, nil)

	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.File}}</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            display: flex;
            {{if eq .TOCPosition "left"}}
            flex-direction: row;
            {{else}}
            flex-direction: row-reverse;
            {{end}}
        }
        .toc-sidebar {
            width: 250px;
            min-width: 150px;
            max-width: 600px;
            background: #f8f9fa;
            border-{{if eq .TOCPosition "left"}}right{{else}}left{{end}}: 1px solid #ddd;
            padding: 20px;
            height: 100vh;
            position: sticky;
            top: 0;
            overflow-y: auto;
            {{if eq .TOCPosition "left"}}
            resize: horizontal;
            {{else}}
            resize: horizontal;
            {{end}}
            overflow: auto;
        }
        .toc-sidebar::-webkit-scrollbar {
            width: 8px;
        }
        .toc-sidebar::-webkit-scrollbar-track {
            background: #f1f1f1;
        }
        .toc-sidebar::-webkit-scrollbar-thumb {
            background: #888;
            border-radius: 4px;
        }
        .toc-sidebar::-webkit-scrollbar-thumb:hover {
            background: #555;
        }
        .toc-sidebar h3 {
            font-size: 0.9em;
            text-transform: uppercase;
            color: #666;
            margin-bottom: 15px;
            letter-spacing: 0.5px;
        }
        .toc-list {
            list-style: none;
        }
        .toc-list li {
            margin: 6px 0;
        }
        .toc-list a {
            color: #333;
            text-decoration: none;
            display: block;
            padding: 4px 0;
            font-size: 0.9em;
            transition: color 0.2s;
        }
        .toc-list a:hover {
            color: #0066cc;
        }
        .toc-level-1 { font-weight: 600; margin-left: 0; }
        .toc-level-2 { margin-left: 16px; }
        .toc-level-3 { margin-left: 32px; }
        .toc-level-4 { margin-left: 48px; font-size: 0.85em; }
        .toc-level-5 { margin-left: 64px; font-size: 0.85em; }
        .toc-level-6 { margin-left: 80px; font-size: 0.85em; }
        .main-content {
            flex: 1;
            max-width: 900px;
            padding: 20px 40px;
            overflow-x: auto;
        }
        .header {
            border-bottom: 1px solid #ddd;
            padding-bottom: 10px;
            margin-bottom: 20px;
        }
        .header a {
            color: #0066cc;
            text-decoration: none;
        }
        .header a:hover {
            text-decoration: underline;
        }
        .content {
            margin-top: 20px;
        }
        .content h1, .content h2, .content h3,
        .content h4, .content h5, .content h6 {
            margin-top: 24px;
            margin-bottom: 16px;
            scroll-margin-top: 20px;
        }
        pre {
            background: #f5f5f5;
            padding: 15px;
            border-radius: 5px;
            overflow-x: auto;
        }
        code {
            background: #f5f5f5;
            padding: 2px 5px;
            border-radius: 3px;
        }
        pre code {
            padding: 0;
        }
        blockquote {
            border-left: 4px solid #ddd;
            margin-left: 0;
            padding-left: 20px;
            color: #666;
            margin: 15px 0;
        }
        table {
            border-collapse: collapse;
            width: 100%;
            margin: 15px 0;
        }
        th, td {
            border: 1px solid #ddd;
            padding: 8px;
            text-align: left;
        }
        th {
            background-color: #f5f5f5;
        }
        @media (max-width: 768px) {
            body {
                flex-direction: column;
            }
            .toc-sidebar {
                width: 100%;
                height: auto;
                position: relative;
                border: none;
                border-bottom: 1px solid #ddd;
            }
        }
    </style>
</head>
<body>
    {{if .Headings}}
    <div class="toc-sidebar">
        <h3>Contents</h3>
        <ul class="toc-list">
        {{range .Headings}}
            <li class="toc-level-{{.Level}}"><a href="#{{.ID}}">{{.Text}}</a></li>
        {{end}}
        </ul>
    </div>
    {{end}}
    <div class="main-content">
        <div class="header">
            <a href="/">← Back to Index</a>
            <h1>{{.File}}</h1>
        </div>
        <div class="content">
            {{.HTMLContent}}
        </div>
    </div>
    <script>
        // Add IDs to headings for anchor links
        document.addEventListener('DOMContentLoaded', function() {
            const headings = document.querySelectorAll('.content h1, .content h2, .content h3, .content h4, .content h5, .content h6');
            const createId = (text) => {
                return text.toLowerCase()
                    .replace(/[^a-z0-9\s-]/g, '')
                    .replace(/\s+/g, '-');
            };

            headings.forEach(heading => {
                if (!heading.id) {
                    heading.id = createId(heading.textContent);
                }
            });

            // Smooth scroll
            document.querySelectorAll('.toc-list a').forEach(anchor => {
                anchor.addEventListener('click', function(e) {
                    e.preventDefault();
                    const target = document.querySelector(this.getAttribute('href'));
                    if (target) {
                        target.scrollIntoView({ behavior: 'smooth' });
                    }
                });
            });
        });
    </script>
</body>
</html>`

	data := struct {
		File        string
		HTMLContent template.HTML
		Headings    []Heading
		TOCPosition string
	}{
		File:        file,
		HTMLContent: template.HTML(htmlContent),
		Headings:    headings,
		TOCPosition: tocPosition,
	}

	t, err := template.New("view").Parse(tmpl)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
	t.Execute(w, data)
}

func main() {
	// Command-line flags
	dir := flag.String("dir", ".", "Directory to serve markdown files from")
	port := flag.String("port", "8080", "Port to serve on")
	toc := flag.String("toc", "left", "Table of contents position: 'left' or 'right'")
	flag.Parse()

	// Set the base directory
	// If there's a positional argument, use it as the directory
	selectedDir := *dir
	if flag.NArg() > 0 {
		selectedDir = flag.Arg(0)
	}

	// Set TOC position
	tocPosition = *toc
	if tocPosition != "left" && tocPosition != "right" {
		log.Printf("Warning: Invalid TOC position '%s', using 'left'", tocPosition)
		tocPosition = "left"
	}

	var err error
	baseDir, err = filepath.Abs(selectedDir)
	if err != nil {
		log.Fatalf("Invalid directory: %v", err)
	}

	// Check if directory exists
	info, err := os.Stat(baseDir)
	if err != nil {
		log.Fatalf("Directory does not exist: %v", err)
	}
	if !info.IsDir() {
		log.Fatalf("Path is not a directory: %s", baseDir)
	}

	// Set up routes
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/view/", viewHandler)

	fmt.Printf("Serving markdown files from: %s\n", baseDir)
	fmt.Printf("Table of contents position: %s\n", tocPosition)
	fmt.Printf("Server running at http://localhost:%s\n", *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
