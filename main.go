package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var (
	Root string

	// Template is an HTML template for a directory listing page/index.html
	Template = template.Must(template.New("index.html").Funcs(template.FuncMap{
		"abs":   abs,
		"clean": filepath.Clean,
	}).Parse(`
<!DOCTYPE html>
<html>
	<head>
		<title>Index &middot; {{ abs .Dir .Root }}</title>
	</head>
	<body>
		{{ $dir := abs .Dir .Root }}
		<ul>
			{{ range .Directories }}
			<li>
				<a href="{{printf "%s/%s" $dir .Name | clean}}">{{.Name}}</a>
			</li>
			{{ end }}
			{{ range .Files }}
			<li>
				<a href="{{printf "%s/%s" $dir .Name | clean}}">{{.Name}}</a>
			</li>
			{{ end }}
		</ul>
		<style>
		ul,ul li{padding:0}a,a:visited{color:#00f}:root{font-size:100%}
		body{font-family:monospace;font-size:1rem}
		ul{list-style-type:none;margin:0}ul li{margin:1em}a{text-decoration:none}
		</style>
	</body>
</html>
`))
)

// Listing is a list of files and directories under a path.
type Listing struct {
	Root        string        `json:"root,omitempty"`
	Dir         string        `json:"dir,omitempty"`
	Directories []os.FileInfo `json:"directories,omitempty"`
	Files       []os.FileInfo `json:"files,omitempty"`
}

func main() {
	var err error

	Root, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	fileServer := http.FileServer(http.Dir(Root))
	mux.Handle("/", http.StripPrefix("/", loggingMiddleware(handleServe(fileServer))))

	fmt.Printf("Serving at address: http://localhost:3000\n")
	err = http.ListenAndServe(":3000", mux)
	log.Fatal(err)
}

func filter(files []os.FileInfo, f func(os.FileInfo) bool) []os.FileInfo {
	vsf := make([]os.FileInfo, 0)
	for _, v := range files {
		if f(v) {
			vsf = append(vsf, v)
		}
	}
	return vsf
}

func filterFiles(files []os.FileInfo) []os.FileInfo {
	// filter out hidden files (.DS_Store, configs, etc.)
	files = filter(files, func(fi os.FileInfo) bool {
		return !strings.HasPrefix(fi.Name(), ".")
	})
	return files
}

func abs(dir, root string) string {
	dir, _ = filepath.Abs(dir)
	root, _ = filepath.Abs(root)
	path := strings.TrimPrefix(dir, root)
	return fmt.Sprintf(filepath.Clean("/" + path))
}

func handleDirectory(w http.ResponseWriter, r *http.Request, path string) {
	contents, err := ioutil.ReadDir(path)
	if err != nil {
		http.Error(w, http.StatusText(500), 500)
		return
	}
	contents = filterFiles(contents)

	directories, files := splitFiles(contents, func(fi os.FileInfo) bool {
		return fi.IsDir()
	})
	list := Listing{
		Root:        Root,
		Dir:         path,
		Directories: directories,
		Files:       files,
	}
	if err := Template.ExecuteTemplate(w, "index.html", list); err != nil {
		log.Println(err.Error())
		http.Error(w, http.StatusText(500), 500)
	}
}

func handleServe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean(r.URL.Path)

		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
		}

		// if we're serving a directory (index.html) short-circuit and return
		// a custom page
		if info.IsDir() {
			ioutil.ReadDir(path)
			handleDirectory(w, r, path)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(abs(filepath.Clean(r.URL.Path), Root), r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

// splits a list of FileInfo objects into two groups based on a boolean predicate
func splitFiles(vs []os.FileInfo, f func(os.FileInfo) bool) ([]os.FileInfo, []os.FileInfo) {
	a := make([]os.FileInfo, 0)
	b := make([]os.FileInfo, 0)
	for _, v := range vs {
		if f(v) {
			a = append(a, v)
		} else {
			b = append(b, v)
		}
	}
	return a, b
}
