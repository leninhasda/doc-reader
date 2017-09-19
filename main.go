package main

import (
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

func main() {

	fs := http.FileServer(http.Dir("./public"))
	http.Handle("/doc/", BasicAuth(http.StripPrefix("/doc/", fs)))
	http.HandleFunc("/", ListAll)
	http.HandleFunc("/index", ListAll)
	http.HandleFunc("/upload", Upload)
	http.ListenAndServe(":5000", nil)
}

// BasicAuth implements a basic http authentication
func BasicAuth(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if ok && validate(u, p) {
			h.ServeHTTP(w, r)
		} else {
			w.Header().Set("WWW-Authenticate", `Basic realm="enter user / pass"`)
			w.WriteHeader(401)
			w.Write([]byte("Unauthorised.\n"))
			return
		}
	})
}
func validate(u, p string) bool {
	if u == "user" && p == "pass" {
		return true
	}
	return false
}

func render(w http.ResponseWriter, filename string, data interface{}) {
	tpl, _ := ioutil.ReadFile(filename)
	tplParsed, _ := template.New("test").Parse(string(tpl))
	tplParsed.Execute(w, data)
}

// ListAll list all existing docs
func ListAll(w http.ResponseWriter, r *http.Request) {
	publicDir := "./public"
	files, err := ioutil.ReadDir(publicDir)
	if err != nil {
		log.Fatal(err)
	}

	data := struct {
		Files []string
	}{}
	for _, file := range files {
		data.Files = append(data.Files, file.Name())
	}
	render(w, "./view/list.html", data)
}

// Upload uploads an api doc
func Upload(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		UploadNew(w, r)
	case "POST":
		Process(w, r)
	}
}

// UploadNew shows the upload form
func UploadNew(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "view/upload-form.html")
}

// ShellExec wrappr around exec.command
func ShellExec(args ...string) (string, string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
		}
	}()

	cmd := exec.Command(args[0], args[1:]...)

	stderr, _ := cmd.StderrPipe()
	stdout, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		panic(err)
	}
	errStr, _ := ioutil.ReadAll(stderr)
	outStr, _ := ioutil.ReadAll(stdout)

	return string(outStr), string(errStr)
}

// Process processes the uploaded file
func Process(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(16 << 20)

	if method := r.Method; method == "POST" {

		file, handle, err := r.FormFile("uploadFile")
		if err != nil {
			fmt.Println(err)
			return
		}
		defer file.Close()

		if validateFileType(file, handle) {
			fileName := strings.Replace(handle.Filename, " ", "-", -1)
			tmpFilePath := "./tmp/" + fileName
			f, err := os.OpenFile(tmpFilePath, os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				fmt.Println(err)
				return
			}
			defer f.Close()

			io.Copy(f, file)

			// parse with aglioß
			renderFilePath := "./public/" + strings.Split(fileName, ".")[0] + ".html"
			shOut, shErr := ShellExec("aglio", "-i", tmpFilePath, "-o", renderFilePath)

			if len(shOut) == 0 && len(shErr) == 0 {
				url := "/doc/" + strings.Replace(fileName, ".apid", ".html", 1)
				http.Redirect(w, r, url, 301)
			} else {
				fmt.Fprintf(w, "out: %s \n\nerr: %s", shOut, shErr)
			}
			return
		}

		fmt.Fprint(w, "File not supported. Only .apid is supported")
		return
	}

	fmt.Fprint(w, "Method not allowed")
}

func validateFileType(file multipart.File, handle *multipart.FileHeader) bool {
	// check extension
	if !strings.HasSuffix(handle.Filename, ".apid") {
		return false
	}

	// check mime type
	allowedType := "text/plain; charset=utf-8"
	b := []byte{}
	file.Read(b)
	if fileType := http.DetectContentType(b); fileType != allowedType {
		return false
	}

	// valid file
	return true
}
