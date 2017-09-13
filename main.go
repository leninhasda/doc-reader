package main

import (
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

func main() {
	fs := http.FileServer(http.Dir("./public"))
	http.Handle("/doc/", http.StripPrefix("/doc/", fs))

	http.HandleFunc("/", ListAll)
	http.HandleFunc("/index", ListAll)
	http.HandleFunc("/upload", Upload)
	http.ListenAndServe(":5000", nil)
}

func render(w http.ResponseWriter, filename string, data interface{}) {
	tpl, _ := ioutil.ReadFile(filename)
	tplParsed, _ := template.New("test").Parse(string(tpl))
	tplParsed.Execute(w, data)
}

// ListAll list all existing docs
func ListAll(w http.ResponseWriter, r *http.Request) {

	render(w, "./view/test.html", nil)
}

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
	err_str, _ := ioutil.ReadAll(stderr)
	out_str, _ := ioutil.ReadAll(stdout)

	return string(out_str), string(err_str)
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
			tmpFilePath := "./tmp/" + handle.Filename
			f, err := os.OpenFile(tmpFilePath, os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				fmt.Println(err)
				return
			}
			defer f.Close()

			io.Copy(f, file)

			// parse with aglio
			renderFilePath := "./public/" + strings.Split(handle.Filename, ".")[0] + ".html"
			shOut, shErr := ShellExec("aglio", "-i", tmpFilePath, "-o", renderFilePath)

			if len(shOut) == 0 && len(shErr) == 0 {
				Redirect(w, r, "/doc/"+strings.Replace(handle.Filename, ".apid", ".html", 1))
				//fmt.Fprintf(w, "done %s", strings.Replace(handle.Filename, ".apid", ".html", 1))
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
	allowed_type := "text/plain; charset=utf-8"
	b := []byte{}
	file.Read(b)
	if fileType := http.DetectContentType(b); fileType != allowed_type {
		return false
	}

	// valid file
	return true
}

func Redirect(w http.ResponseWriter, r *http.Request, url string) {

	http.Redirect(w, r, url, 301)
}
