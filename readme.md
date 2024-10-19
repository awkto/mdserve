# Mini Markdown Server
**mdserve.go** is meant to be a tiny webserver to quickly serve your markdown files in a webpage

Additionally if you have any sensitive notes you can use gpg to encrypt them. 
- **mdserve.go** will automatically scan and decrypt gpg encrypted files
- **mdserve.go** looks for your gpg password in a file .secret.key

### Features
- Editing of markdown files live in web page
- Password protection of webpage also via .secret.key (username admin)

# Setup

### Install golang and gpg
```bash
sudo apt install golang -y
sudo apt install gpg -y
```

### Initialize
```bash
go mod init markdown_server
go get github.com/gomarkdown/markdown
```

# Run webserver

1. Clone Repo
2. Create file and add your password into **.secret.key**
3. Serve with `go run mdserve.go`
4. Point your browser to **http://localhost:8080**
5. For specific files such as howto.md use path **http://localhost:8080/howto.md**

