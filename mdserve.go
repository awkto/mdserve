package main

import (
    "fmt"
    "html/template"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "github.com/gomarkdown/markdown"
)

// Display the Markdown file as HTML or in the editor.
func markdownHandler(w http.ResponseWriter, r *http.Request) {
    file := r.URL.Path[1:]
    if file == "" {
        file = "index.md"
    }

    if r.Method == http.MethodPost {
        // Handle file saving when the user submits the form.
        newContent := r.FormValue("content")
        err := ioutil.WriteFile(file, []byte(newContent), 0644)
        if err != nil {
            http.Error(w, "Could not save file", http.StatusInternalServerError)
            return
        }
    }

    content, err := ioutil.ReadFile(file)
    if err != nil {
        http.Error(w, "File not found", http.StatusNotFound)
        return
    }

    // Render the Markdown content as HTML.
    htmlContent := markdown.ToHTML(content, nil, nil)

    tmpl := `
    <html>
    <body>
        <h1>Edit Markdown File</h1>
        <form method="POST" action="/">
            <textarea name="content" rows="20" cols="80">{{.RawContent}}</textarea><br>
            <input type="submit" value="Save">
        </form>
        <h1>Preview</h1>
        <div>{{.HTMLContent}}</div>
    </body>
    </html>`
    
    data := struct {
        RawContent  string
        HTMLContent template.HTML
    }{
        RawContent:  string(content),
        HTMLContent: template.HTML(htmlContent),
    }

    t, _ := template.New("editor").Parse(tmpl)
    t.Execute(w, data)
}

func main() {
    port := "8080"
    if len(os.Args) > 1 {
        port = os.Args[1]
    }
    http.HandleFunc("/", markdownHandler)
    fmt.Printf("Serving on http://localhost:%s\n", port)
    log.Fatal(http.ListenAndServe(":"+port, nil))
}
