package main

import (
    "fmt"
    "html/template"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "os/exec"
    "github.com/gomarkdown/markdown"
)

const encryptionPassword = "your-secure-password" // Static password for GPG encryption
const adminUsername = "admin"                    // Static username for web login
const adminPassword = "password123"              // Static password for web login

// Check Basic Authentication credentials
func checkAuth(r *http.Request) bool {
    username, password, ok := r.BasicAuth()
    return ok && username == adminUsername && password == adminPassword
}

// Encrypt the file using GPG
func encryptFile(file string) error {
    cmd := exec.Command("gpg", "--batch", "--yes", "--passphrase", encryptionPassword, "-c", file)
    err := cmd.Run()
    if err != nil {
        return fmt.Errorf("GPG encryption failed: %v", err)
    }
    return nil
}

// Handle Markdown viewing with authentication
func viewHandler(w http.ResponseWriter, r *http.Request) {
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
    tmpl := `
    <html>
    <body>
        <a href="/edit/{{.File}}">Edit this file</a>
        <h1>Preview</h1>
        <div>{{.HTMLContent}}</div>
    </body>
    </html>`

    data := struct {
        File        string
        HTMLContent template.HTML
    }{
        File:        file,
        HTMLContent: template.HTML(htmlContent),
    }

    t, _ := template.New("view").Parse(tmpl)
    t.Execute(w, data)
}

// Handle Markdown editing with authentication
func editHandler(w http.ResponseWriter, r *http.Request) {
    if !checkAuth(r) {
        w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
        http.Error(w, "Unauthorized.", http.StatusUnauthorized)
        return
    }

    file := r.URL.Path[len("/edit/"):]
    if file == "" {
        http.Error(w, "File not specified", http.StatusBadRequest)
        return
    }

    if r.Method == http.MethodPost {
        newContent := r.FormValue("content")
        err := ioutil.WriteFile(file, []byte(newContent), 0644)
        if err != nil {
            http.Error(w, "Could not save file", http.StatusInternalServerError)
            return
        }

        // Encrypt the file after saving
        err = encryptFile(file)
        if err != nil {
            log.Printf("Encryption error: %v", err)
            http.Error(w, "Encryption failed", http.StatusInternalServerError)
            return
        }

        http.Redirect(w, r, "/"+file, http.StatusSeeOther)
        return
    }

    content, err := ioutil.ReadFile(file)
    if err != nil {
        http.Error(w, "File not found", http.StatusNotFound)
        return
    }

    tmpl := `
    <html>
    <body>
        <h1>Edit {{.File}}</h1>
        <form method="POST" action="/edit/{{.File}}">
            <textarea name="content" rows="20" cols="80">{{.RawContent}}</textarea><br>
            <input type="submit" value="Save">
        </form>
        <a href="/{{.File}}">Cancel</a>
    </body>
    </html>`

    data := struct {
        File       string
        RawContent string
    }{
        File:       file,
        RawContent: string(content),
    }

    t, _ := template.New("edit").Parse(tmpl)
    t.Execute(w, data)
}

func main() {
    port := "8080"
    if len(os.Args) > 1 {
        port = os.Args[1]
    }

    http.HandleFunc("/", viewHandler)               // View route
    http.HandleFunc("/edit/", editHandler)          // Edit route

    fmt.Printf("Serving on http://localhost:%s\n", port)
    log.Fatal(http.ListenAndServe(":"+port, nil))
}

