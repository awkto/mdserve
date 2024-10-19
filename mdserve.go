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

func markdownHandler(w http.ResponseWriter, r *http.Request) {
    file := r.URL.Path[1:] // Get the file path from the URL
    if file == "" {
        file = "index.md" // Default to index.md if no file is specified
    }

    content, err := ioutil.ReadFile(file)
    if err != nil {
        http.Error(w, "File not found", http.StatusNotFound)
        return
    }

    htmlContent := markdown.ToHTML(content, nil, nil)
    tmpl := `<html><body>{{.}}</body></html>`
    t, _ := template.New("webpage").Parse(tmpl)
    t.Execute(w, template.HTML(htmlContent))
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
