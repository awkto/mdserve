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
	"sort"
	"strings"

	"github.com/gomarkdown/markdown"
)

var baseDir string

// FileInfo represents a file or directory for the index
type FileInfo struct {
	Name        string
	Path        string
	IsDirectory bool
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

	// Convert markdown to HTML
	htmlContent := markdown.ToHTML(content, nil, nil)

	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.File}}</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            max-width: 900px;
            margin: 0 auto;
            padding: 20px;
            line-height: 1.6;
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
    </style>
</head>
<body>
    <div class="header">
        <a href="/">← Back to Index</a>
        <h1>{{.File}}</h1>
    </div>
    <div class="content">
        {{.HTMLContent}}
    </div>
</body>
</html>`

	data := struct {
		File        string
		HTMLContent template.HTML
	}{
		File:        file,
		HTMLContent: template.HTML(htmlContent),
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
	flag.Parse()

	// Set the base directory
	// If there's a positional argument, use it as the directory
	selectedDir := *dir
	if flag.NArg() > 0 {
		selectedDir = flag.Arg(0)
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
	fmt.Printf("Server running at http://localhost:%s\n", *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
