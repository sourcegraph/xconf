package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
)

var (
	addr     = flag.String("http", ":5400", "HTTP bind address")
	dev      = flag.Bool("dev", false, "development mode")
	sgURLStr = flag.String("sg", "https://sourcegraph.com", "base Sourcegraph URL")
)

func main() {
	log.SetFlags(0)
	flag.Parse()

	sgURL, err := url.Parse(*sgURLStr)
	if err != nil {
		log.Fatal(err)
	}
	sgc.BaseURL = sgURL.ResolveReference(&url.URL{Path: "/api/"})
	sgc.UserAgent = "xconf/0.0.1"

	if err := parseTemplates(); err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", homeHandler)

	log.Println("Listening on", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

var (
	tmpl      *template.Template
	tmplMu    sync.Mutex
	tmplFuncs = template.FuncMap{
		"exampleQueries": func() []string { return exampleQueries },
		"queryURL": func(q string) string {
			return "/?" + url.Values{"q": []string{q}}.Encode()
		},
	}
)

func parseTemplates() error {
	tmplMu.Lock()
	defer tmplMu.Unlock()
	var err error
	tmpl, err = template.New("").Funcs(tmplFuncs).ParseGlob("tmpl/*")
	return err
}

var (
	sgc = sourcegraph.NewClient(nil)
)

var (
	exampleQueries = []string{
		"nodejs",
		"docpad",
		"mysql",
		"postgres",
		"wordpress",
		"ubuntu",
		"apt-get install",
		"add-apt-repository",
		"golang",
	}
)

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if *dev {
		if err := parseTemplates(); err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := r.ParseForm(); err != nil {
		log.Println("ParseForm:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var data struct {
		Query   string
		Results []string
	}
	data.Query = strings.TrimSpace(r.FormValue("q"))

	if data.Query != "" {
		opt := &sourcegraph.UnitListOptions{Query: data.Query}
		units, _, err := sgc.Units.List(opt)
		if err != nil {
			log.Println("Units.List:", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, u := range units {
			su, err := u.SourceUnit()
			if err != nil {
				log.Println("SourceUnit:", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			sourceboxURL := sgc.BaseURL.ResolveReference(&url.URL{
				Path: fmt.Sprintf("/%s@%s/.tree/%s/.sourcebox.js", u.Repo, u.CommitID, su.Files[0]),
			})
			data.Results = append(data.Results, sourceboxURL.String())
		}
	}

	if err := tmpl.ExecuteTemplate(w, "home.html", &data); err != nil {
		log.Println("ExecuteTemplate:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
