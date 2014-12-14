package main

import (
	"encoding/json"
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
	addr          = flag.String("http", ":5400", "HTTP bind address")
	dev           = flag.Bool("dev", false, "development mode")
	sgURLStr      = flag.String("sg", "https://sourcegraph.com", "base Sourcegraph URL")
	sgAssetURLStr = flag.String("sg-asset", "https://sourcegraph.com/static/", "base Sourcegraph asset URL")
)

var (
	sgURL      *url.URL
	sgAssetURL *url.URL
)

func main() {
	log.SetFlags(0)
	flag.Parse()

	var err error
	sgURL, err = url.Parse(*sgURLStr)
	if err != nil {
		log.Fatal(err)
	}
	*sgURLStr = sgURL.String()
	sgc.BaseURL = sgURL.ResolveReference(&url.URL{Path: "/api/"})
	sgc.UserAgent = "xconf/0.0.1"

	sgAssetURL, err = url.Parse(*sgAssetURLStr)
	if err != nil {
		log.Fatal(err)
	}
	*sgAssetURLStr = sgAssetURL.String()

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
		"sgAssetURL": func(path string) string {
			return sgAssetURL.ResolveReference(&url.URL{Path: path}).String()
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
	httpClient = &http.Client{}
	sgc        = sourcegraph.NewClient(httpClient)
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
		"go get",
		"",
		"RUN",
		"ONBUILD",
		"ENV",
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
		Results []*sourcegraph.Sourcebox
	}
	data.Query = strings.TrimSpace(r.FormValue("q"))

	if data.Query != "" {
		opt := &sourcegraph.UnitListOptions{
			Query:       data.Query,
			UnitType:    "Dockerfile",
			ListOptions: sourcegraph.ListOptions{PerPage: 4},
		}
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
				Path: fmt.Sprintf("/%s@%s/.tree/%s/.sourcebox.json", u.Repo, u.CommitID, su.Files[0]),
			})

			resp, err := httpClient.Get(sourceboxURL.String())
			if err != nil {
				log.Printf("Get %q: %s", sourceboxURL, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer resp.Body.Close()

			var sb *sourcegraph.Sourcebox
			if err := json.NewDecoder(resp.Body).Decode(&sb); err != nil {
				log.Println("Decode:", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			data.Results = append(data.Results, sb)
		}
	}

	var tmplFile string
	if r.Header.Get("x-pjax") != "" {
		tmplFile = "results.inc.html"
	} else {
		tmplFile = "home.html"
	}

	if err := tmpl.ExecuteTemplate(w, tmplFile, &data); err != nil {
		log.Println("ExecuteTemplate:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
