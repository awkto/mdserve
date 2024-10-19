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

// Function to check if the provided username and password match
func checkAuth(r *http.Request) bool {
    username, password, ok := r.BasicAuth()
    return ok && username == "alia" && password == "melange"
}

func markdownHandler(w http.ResponseWriter, r *http.Request) {
    if !checkAuth(r) {
        w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
        http.Error(w, "Unauthorized.", http.StatusUnauthorized)
        return
    }

    file := r.URL.Path[1:]
    if file == "" {
        file = "index.md"
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
