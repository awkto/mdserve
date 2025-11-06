package main

import (
	"encoding/json"
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
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
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

// Generate ID from text - matches gomarkdown's AutoHeadingIDs behavior
func generateHeadingID(text string) string {
	// Convert to lowercase
	id := strings.ToLower(text)
	// Replace non-alphanumeric characters (except spaces, hyphens) with hyphens
	// This matches gomarkdown's behavior more closely
	id = regexp.MustCompile(`[^\w\s-]+`).ReplaceAllString(id, "-")
	// Replace spaces and underscores with hyphens
	id = regexp.MustCompile(`[\s_]+`).ReplaceAllString(id, "-")
	// Remove any consecutive hyphens
	id = regexp.MustCompile(`-+`).ReplaceAllString(id, "-")
	id = strings.Trim(id, "-")
	return id
}

// Fix indented code blocks in list items
// The gomarkdown parser has a bug where fenced code blocks with 2-space indentation
// in list items are not recognized if they contain blank lines. This function
// adds extra indentation to such code blocks to work around the issue.
func fixIndentedCodeBlocks(content []byte) []byte {
	lines := strings.Split(string(content), "\n")
	var result []string
	inCodeBlock := false
	codeBlockIndent := 0
	
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		
		// Detect start of fenced code block in list continuation
		if !inCodeBlock {
			if match := regexp.MustCompile(`^(\s+)` + "`" + `{3,}(\w*)`).FindStringSubmatch(line); match != nil {
				indent := len(match[1])
				// Check if this is likely part of a list item (2 or 3 space indent)
				if indent >= 2 && indent <= 4 {
					inCodeBlock = true
					codeBlockIndent = indent
					// Add 2 more spaces to ensure it's recognized as list content
					result = append(result, "  "+line)
					continue
				}
			}
		} else {
			// Inside code block - check for end fence
			if match := regexp.MustCompile(`^(\s+)` + "`" + `{3,}$`).FindStringSubmatch(line); match != nil {
				indent := len(match[1])
				if indent == codeBlockIndent {
					// End of code block
					result = append(result, "  "+line)
					inCodeBlock = false
					codeBlockIndent = 0
					continue
				}
			}
			// Regular code block content or blank line - add 2 spaces
			if inCodeBlock && codeBlockIndent > 0 {
				result = append(result, "  "+line)
				continue
			}
		}
		
		result = append(result, line)
	}
	
	return []byte(strings.Join(result, "\n"))
}

// Extract headings from markdown content
func extractHeadings(content []byte) []Heading {
	var headings []Heading
	lines := strings.Split(string(content), "\n")
	headingRegex := regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	explicitIDRegex := regexp.MustCompile(`\s*\{#([^}]+)\}\s*$`)
	codeBlockRegex := regexp.MustCompile(`^\s*` + "`" + `{3,}`)

	// Track used IDs to handle duplicates
	usedIDs := make(map[string]int)

	inCodeBlock := false
	for _, line := range lines {
		// Check if we're entering or exiting a code block
		if codeBlockRegex.MatchString(line) {
			inCodeBlock = !inCodeBlock
			continue
		}

		// Skip processing if we're inside a code block
		if inCodeBlock {
			continue
		}

		line = strings.TrimSpace(line)
		if matches := headingRegex.FindStringSubmatch(line); matches != nil {
			level := len(matches[1])
			rawText := strings.TrimSpace(matches[2])

			// Check for explicit ID in format {#id}
			var id string
			if idMatches := explicitIDRegex.FindStringSubmatch(rawText); idMatches != nil {
				id = idMatches[1]
				// Remove the {#id} part from the text
				rawText = explicitIDRegex.ReplaceAllString(rawText, "")
				rawText = strings.TrimSpace(rawText)
			}

			cleanText := cleanMarkdown(rawText)

			// Generate ID to match gomarkdown's AutoHeadingIDs if no explicit ID
			if id == "" {
				id = generateHeadingID(cleanText)
			}

			// Prefix numeric IDs to match the JavaScript fix in the rendered HTML
			// HTML IDs cannot start with a digit
			if len(id) > 0 && id[0] >= '0' && id[0] <= '9' {
				id = "heading-" + id
			}

			// Handle duplicate IDs by appending a suffix
			originalID := id
			if count, exists := usedIDs[originalID]; exists {
				// This ID has been used before, append a number
				usedIDs[originalID] = count + 1
				id = fmt.Sprintf("%s-%d", originalID, count)
			} else {
				// First time seeing this ID
				usedIDs[originalID] = 1
			}

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

	// Fix indented code blocks before parsing
	content = fixIndentedCodeBlocks(content)

	// Extract headings for TOC
	headings := extractHeadings(content)

	// Convert markdown to HTML with AutoHeadingIDs extension
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)
	// Disable Smartypants to prevent backticks from being converted to smart quotes
	htmlFlags := html.CommonFlags &^ html.Smartypants
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)
	htmlContent := markdown.ToHTML(content, p, renderer)

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
            display: flex;
            align-items: center;
            justify-content: space-between;
        }
        .toc-controls {
            display: flex;
            gap: 8px;
        }
        .toc-control-btn {
            width: 20px;
            height: 20px;
            border-radius: 50%;
            border: 1px solid #999;
            background: transparent;
            cursor: pointer;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 14px;
            color: #666;
            padding: 0;
            transition: all 0.2s;
        }
        .toc-control-btn:hover {
            background: #e8e8e8;
            border-color: #666;
            color: #333;
        }
        .toc-list {
            list-style: none;
        }
        .toc-list li {
            margin: 6px 0;
            position: relative;
        }
        .toc-item {
            display: flex;
            align-items: center;
        }
        .toc-toggle {
            cursor: pointer;
            width: 16px;
            height: 16px;
            margin-right: 4px;
            flex-shrink: 0;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            color: #666;
            font-size: 12px;
            user-select: none;
        }
        .toc-toggle:hover {
            color: #0066cc;
        }
        .toc-toggle.empty {
            visibility: hidden;
        }
        .toc-children {
            list-style: none;
            padding-left: 20px;
            margin-top: 4px;
            border-left: 1px solid #ddd;
            margin-left: 8px;
        }
        .toc-children.collapsed {
            display: none;
        }
        .toc-list a {
            color: #333;
            text-decoration: none;
            display: block;
            padding: 4px 0;
            font-size: 0.9em;
            transition: color 0.2s;
            flex: 1;
            font-weight: normal;
        }
        .toc-list a:hover {
            color: #0066cc;
        }
        .toc-level-1 { padding-left: 0; }
        .toc-level-1 > .toc-item > a { font-weight: 600; }
        .toc-level-2 { padding-left: 0; }
        .toc-level-3 { padding-left: 0; }
        .toc-level-4 { padding-left: 0; }
        .toc-level-5 { padding-left: 0; }
        .toc-level-6 { padding-left: 0; }
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
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .header-left {
            display: flex;
            flex-direction: column;
        }
        .header a {
            color: #0066cc;
            text-decoration: none;
        }
        .header a:hover {
            text-decoration: underline;
        }
        .toggle-btn {
            position: fixed;
            top: 20px;
            right: 20px;
            padding: 10px 20px;
            background: #0066cc;
            color: white;
            border: none;
            border-radius: 6px;
            cursor: pointer;
            font-size: 0.9em;
            font-weight: 500;
            transition: all 0.2s;
            box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
            z-index: 1000;
        }
        .toggle-btn:hover {
            background: #0052a3;
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
            transform: translateY(-1px);
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
        .content h1 {
            font-size: 2em;
            border-bottom: 1px solid #ddd;
            padding-bottom: 0.3em;
        }
        .content h2 {
            font-size: 1.5em;
            border-bottom: 1px solid #eee;
            padding-bottom: 0.3em;
        }
        .content h3 {
            font-size: 1.25em;
        }
        .content h4 {
            font-size: 1.1em;
        }
        .content h5 {
            font-size: 1.05em;
        }
        .content h6 {
            font-size: 1em;
            font-weight: 600;
        }
        .raw-source {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100vw;
            height: 100vh;
            background: #1e1e1e;
            color: #d4d4d4;
            padding: 60px 40px 40px 40px;
            white-space: pre-wrap;
            font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', 'Consolas', monospace;
            font-size: 14px;
            line-height: 1.6;
            overflow: auto;
            z-index: 999;
            box-sizing: border-box;
        }
        .raw-source::-webkit-scrollbar {
            width: 12px;
            height: 12px;
        }
        .raw-source::-webkit-scrollbar-track {
            background: #252526;
        }
        .raw-source::-webkit-scrollbar-thumb {
            background: #424242;
            border-radius: 6px;
        }
        .raw-source::-webkit-scrollbar-thumb:hover {
            background: #4e4e4e;
        }
        /* Markdown syntax highlighting */
        .md-heading { color: #569cd6; font-weight: bold; }
        .md-bold { color: #ce9178; font-weight: bold; }
        .md-italic { color: #ce9178; font-style: italic; }
        .md-code { color: #d16969; background: #2d2d2d; padding: 2px 4px; border-radius: 3px; }
        .md-code-block { color: #d16969; background: #2d2d2d; display: block; padding: 10px; border-radius: 4px; margin: 10px 0; }
        .md-link { color: #4ec9b0; }
        .md-list { color: #c586c0; }
        .md-quote { color: #6a9955; border-left: 3px solid #6a9955; padding-left: 10px; margin: 10px 0; display: block; }
        .md-hr { color: #464646; }
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
        .content ul, .content ol {
            margin-left: 20px;
            padding-left: 20px;
            margin-top: 10px;
            margin-bottom: 10px;
        }
        .content li {
            margin: 5px 0;
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
        <h3>
            <span>Contents</span>
            <div class="toc-controls">
                <button class="toc-control-btn" onclick="expandAllToc()" title="Expand all">+</button>
                <button class="toc-control-btn" onclick="collapseAllToc()" title="Collapse all">−</button>
            </div>
        </h3>
        <ul class="toc-list" id="toc-root">
        </ul>
    </div>
    {{end}}
    <button class="toggle-btn" onclick="toggleView()">Show Source</button>
    <div class="main-content">
        <div class="header">
            <div class="header-left">
                <a href="/">← Back to Index</a>
                <h1>{{.File}}</h1>
            </div>
        </div>
        <div class="content" id="rendered-content">
            {{.HTMLContent}}
        </div>
    </div>
    <div class="raw-source" id="raw-content"></div>
    <textarea id="raw-markdown-data" style="display:none;">{{.RawContent}}</textarea>
    <script>
        let isShowingSource = false;
        const rawMarkdown = document.getElementById('raw-markdown-data').value;

        // Syntax highlight markdown
        function highlightMarkdown(text) {
            var escapeMap = {'&': '&amp;', '<': '&lt;', '>': '&gt;'};
            text = text.replace(/[&<>]/g, function(m) { return escapeMap[m]; });

            // Code blocks (triple backtick) - must be at start of line
            // Use a placeholder to protect code blocks from other replacements
            var codeBlocks = [];
            text = text.replace(/^(\x60\x60\x60[^\n]*\n)([\s\S]*?)^(\x60\x60\x60)$/gm, function(match, open, content, close) {
                var placeholder = '___CODE_BLOCK_' + codeBlocks.length + '___';
                codeBlocks.push('<span class="md-code-block">' + open + content + close + '</span>');
                return placeholder;
            });

            // Inline code (single backtick)
            text = text.replace(/\x60([^\x60\n]+)\x60/g, '<span class="md-code">$&</span>');

            // Headings (only outside of code blocks, now that they're replaced with placeholders)
            text = text.replace(/^(#{1,6}\s+.+)$/gm, '<span class="md-heading">$1</span>');

            // Bold
            text = text.replace(/(\*\*|__)([^*_\n]+)(\*\*|__)/g, '<span class="md-bold">$1$2$3</span>');

            // Italic
            text = text.replace(/(\*|_)([^*_\n]+)(\*|_)/g, '<span class="md-italic">$1$2$3</span>');

            // Links
            text = text.replace(/\[([^\]]+)\]\([^\)]+\)/g, '<span class="md-link">$&</span>');

            // List items
            text = text.replace(/^(\s*[-*+]\s+)/gm, '<span class="md-list">$1</span>');

            // Blockquotes
            text = text.replace(/^(&gt;.+)$/gm, '<span class="md-quote">$1</span>');

            // Horizontal rules
            text = text.replace(/^([-*_]{3,})$/gm, '<span class="md-hr">$1</span>');

            // Restore code blocks from placeholders
            for (var i = 0; i < codeBlocks.length; i++) {
                text = text.replace('___CODE_BLOCK_' + i + '___', codeBlocks[i]);
            }

            return text;
        }

        // Shared scroll position
        let sharedScrollPos = 0;
        let highlightedContent = null;

        // Expand/Collapse all TOC functions
        function expandAllToc() {
            document.querySelectorAll('.toc-children.collapsed').forEach(child => {
                child.classList.remove('collapsed');
            });
            document.querySelectorAll('.toc-toggle').forEach(toggle => {
                if (!toggle.classList.contains('empty')) {
                    toggle.textContent = '▼';
                }
            });
        }

        function collapseAllToc() {
            document.querySelectorAll('.toc-children').forEach(child => {
                child.classList.add('collapsed');
            });
            document.querySelectorAll('.toc-toggle').forEach(toggle => {
                if (!toggle.classList.contains('empty')) {
                    toggle.textContent = '▶';
                }
            });
        }

        // Toggle between rendered and source view
        function toggleView() {
            const renderedWrapper = document.querySelector('.main-content');
            const tocSidebar = document.querySelector('.toc-sidebar');
            const rawContent = document.getElementById('raw-content');
            const toggleBtn = document.querySelector('.toggle-btn');

            if (isShowingSource) {
                // Currently showing source, switch to rendered
                // Save current scroll position from source (it scrolls on itself)
                sharedScrollPos = rawContent.scrollTop;

                // Hide source, show rendered
                rawContent.style.display = 'none';
                if (tocSidebar) tocSidebar.style.display = 'block';
                renderedWrapper.style.display = 'block';
                toggleBtn.textContent = 'Show Source';
                isShowingSource = false;

                // Restore scroll position to window (rendered view scrolls on window)
                requestAnimationFrame(function() {
                    window.scrollTo(0, sharedScrollPos);
                });
            } else {
                // Currently showing rendered, switch to source
                // Save current scroll position from window (rendered view scrolls on window)
                sharedScrollPos = window.pageYOffset || document.documentElement.scrollTop;

                // Populate highlighted content if not already done
                if (!highlightedContent) {
                    highlightedContent = highlightMarkdown(rawMarkdown);
                }

                // Show source first, THEN set content and scroll
                rawContent.style.display = 'block';
                rawContent.innerHTML = highlightedContent;

                // Hide rendered
                renderedWrapper.style.display = 'none';
                if (tocSidebar) tocSidebar.style.display = 'none';
                toggleBtn.textContent = 'Show Rendered';
                isShowingSource = true;

                // Restore scroll position to rawContent (it scrolls on itself)
                requestAnimationFrame(function() {
                    rawContent.scrollTop = sharedScrollPos;
                });
            }
        }

        // Add IDs to headings that don't have them (for explicit {#id} syntax)
        // The markdown renderer already adds IDs for regular headings
        document.addEventListener('DOMContentLoaded', function() {
            // STEP 1: Fix all heading IDs first
            const headings = document.querySelectorAll('.content h1, .content h2, .content h3, .content h4, .content h5, .content h6');
            const explicitIdRegex = /\s*\{#([^}]+)\}\s*$/;
            const usedIDs = {};

            headings.forEach(heading => {
                const originalText = heading.textContent.trim();

                // Check if text has explicit ID marker {#id} - need to clean it up
                if (explicitIdRegex.test(originalText)) {
                    // The markdown library doesn't handle {#id} syntax, so we need to
                    const match = explicitIdRegex.exec(originalText);
                    if (match && !heading.id) {
                        heading.id = match[1];
                    }
                    // Remove {#id} from display text
                    heading.textContent = originalText.replace(explicitIdRegex, '').trim();
                }

                // Fix IDs that start with numbers (invalid HTML IDs)
                // querySelector can't handle IDs starting with digits
                if (heading.id && /^[0-9]/.test(heading.id)) {
                    console.log('Fixing numeric ID:', heading.id, '->', 'heading-' + heading.id);
                    heading.id = 'heading-' + heading.id;
                }

                // Handle duplicate IDs by appending a suffix
                if (heading.id) {
                    const originalID = heading.id;
                    if (usedIDs[originalID]) {
                        // This ID has been used before, append a number
                        const newID = originalID + '-' + usedIDs[originalID];
                        console.log('Duplicate ID detected:', originalID, '->', newID);
                        heading.id = newID;
                        usedIDs[originalID]++;
                    } else {
                        // First time seeing this ID
                        usedIDs[originalID] = 1;
                    }
                }
            });

            // STEP 2: Build hierarchical TOC (uses the fixed IDs from headingsJSON)
            const tocData = {{.HeadingsJSON}};
            const tocRoot = document.getElementById('toc-root');
            
            if (tocRoot && tocData.length > 0) {
                function buildTocTree(headings) {
                    const root = [];
                    const stack = [{ level: 0, children: root }];
                    
                    headings.forEach(heading => {
                        const item = {
                            level: heading.Level,
                            text: heading.Text,
                            id: heading.ID,
                            children: []
                        };
                        
                        // Pop stack until we find the parent level
                        while (stack.length > 1 && stack[stack.length - 1].level >= heading.Level) {
                            stack.pop();
                        }
                        
                        // Add to parent's children
                        stack[stack.length - 1].children.push(item);
                        
                        // Push this item onto stack for potential children
                        stack.push(item);
                    });
                    
                    return root;
                }
                
                function createTocElement(item) {
                    const li = document.createElement('li');
                    li.className = 'toc-level-' + item.level;
                    
                    const itemDiv = document.createElement('div');
                    itemDiv.className = 'toc-item';
                    
                    // Create toggle button if has children
                    const toggle = document.createElement('span');
                    toggle.className = 'toc-toggle';
                    if (item.children.length > 0) {
                        toggle.textContent = '▼';
                        toggle.onclick = function(e) {
                            e.stopPropagation();
                            const childrenUl = li.querySelector('.toc-children');
                            if (childrenUl) {
                                childrenUl.classList.toggle('collapsed');
                                toggle.textContent = childrenUl.classList.contains('collapsed') ? '▶' : '▼';
                            }
                        };
                    } else {
                        toggle.classList.add('empty');
                    }
                    
                    // Create link
                    const link = document.createElement('a');
                    link.href = '#' + item.id;
                    link.textContent = item.text;
                    
                    itemDiv.appendChild(toggle);
                    itemDiv.appendChild(link);
                    li.appendChild(itemDiv);
                    
                    // Add children if any
                    if (item.children.length > 0) {
                        const childrenUl = document.createElement('ul');
                        childrenUl.className = 'toc-children';
                        item.children.forEach(child => {
                            childrenUl.appendChild(createTocElement(child));
                        });
                        li.appendChild(childrenUl);
                    }
                    
                    return li;
                }
                
                const tree = buildTocTree(tocData);
                tree.forEach(item => {
                    tocRoot.appendChild(createTocElement(item));
                });
            }

            // Smooth scroll
            document.querySelectorAll('.toc-list a').forEach(anchor => {
                anchor.addEventListener('click', function(e) {
                    e.preventDefault();
                    const href = this.getAttribute('href');
                    // Extract ID from href (remove the # prefix)
                    const targetId = href.substring(1);
                    console.log('Looking for heading with ID:', targetId);
                    const target = document.getElementById(targetId);
                    if (target) {
                        console.log('Found target, scrolling to:', target);
                        target.scrollIntoView({ behavior: 'smooth' });
                    } else {
                        console.error('Target heading not found:', targetId);
                    }
                });
            });
        });
    </script>
</body>
</html>`

	// Convert headings to JSON for JavaScript
	headingsJSON, err := json.Marshal(headings)
	if err != nil {
		headingsJSON = []byte("[]")
	}

	data := struct {
		File         string
		HTMLContent  template.HTML
		RawContent   string
		Headings     []Heading
		HeadingsJSON template.JS
		TOCPosition  string
	}{
		File:         file,
		HTMLContent:  template.HTML(htmlContent),
		RawContent:   string(content),
		Headings:     headings,
		HeadingsJSON: template.JS(headingsJSON),
		TOCPosition:  tocPosition,
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
