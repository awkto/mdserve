package main

import (
    "bufio"
    "fmt"
    "html/template"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "github.com/gomarkdown/markdown"
)

var encryptionPassword string // Holds the password fetched from the file
const adminUsername = "alia" // Admin username

// Read the password from a file
func readPasswordFromFile(filePath string) (string, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return "", fmt.Errorf("could not open password file: %v", err)
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    if scanner.Scan() {
        return scanner.Text(), nil
    }
    return "", fmt.Errorf("password file is empty")
}

// Decrypt all GPG files at startup
func decryptAllGPGFiles() error {
    err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if strings.HasSuffix(path, ".gpg") {
            outputFile := strings.TrimSuffix(path, ".gpg")
            cmd := exec.Command("gpg", "--batch", "--yes", "--passphrase", encryptionPassword, 
                                "-o", outputFile, "-d", path)
            if err := cmd.Run(); err != nil {
                return fmt.Errorf("Failed to decrypt %s: %v", path, err)
            }
            log.Printf("Decrypted: %s", path)
        }
        return nil
    })
    return err
}

// Basic authentication check
func checkAuth(r *http.Request) bool {
    username, password, ok := r.BasicAuth()
    return ok && username == adminUsername && password == encryptionPassword
}

// Encrypt a file using GPG
func encryptFile(file string) error {
    cmd := exec.Command("gpg", "--batch", "--yes", "--passphrase", encryptionPassword, "-c", file)
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("GPG encryption failed: %v", err)
    }
    return nil
}

// View handler with authentication
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

// Edit handler with authentication
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
    // Read password from the specified file
    var err error
    encryptionPassword, err = readPasswordFromFile(".secret.key")
    if err != nil {
        log.Fatalf("Failed to read password: %v", err)
    }

    // Decrypt all GPG files at startup
    if err := decryptAllGPGFiles(); err != nil {
        log.Fatalf("Failed to decrypt files: %v", err)
    }

    port := "8080"
    if len(os.Args) > 1 {
        port = os.Args[1]
    }

    http.HandleFunc("/", viewHandler)               // View route
    http.HandleFunc("/edit/", editHandler)          // Edit route

    fmt.Printf("Serving on http://localhost:%s\n", port)
    log.Fatal(http.ListenAndServe(":"+port, nil))
}

