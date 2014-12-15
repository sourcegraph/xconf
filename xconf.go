package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gregjones/httpcache"

	"sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
)

var (
	addr          = flag.String("http", ":5400", "HTTP bind address")
	dev           = flag.Bool("dev", false, "development mode")
	sgURLStr      = flag.String("sg", "https://sourcegraph.com", "base Sourcegraph URL")
	sgAssetURLStr = flag.String("sg-asset", "https://sourcegraph.com/static/", "base Sourcegraph asset URL")

	clientTimeout = flag.Duration("client-timeout", time.Second*5, "timeout for HTTP requests")
	queryTimeout  = flag.Duration("query-timeout", time.Second*7, "timeout for query API call")
)

var (
	sgURL      *url.URL
	sgAssetURL *url.URL
)

var (
	httpClient = &http.Client{}
	sgc        = sourcegraph.NewClient(httpClient)
)

func main() {
	log.SetFlags(0)
	flag.Parse()

	httpClient.Timeout = *clientTimeout

	if !*dev {
		httpClient.Transport = newCancelableHTTPMemoryCacheTransport()
	}

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
	http.HandleFunc("/robots.txt", func(w http.ResponseWriter, req *http.Request) {
		io.WriteString(w, `User-agent: *
Allow: /
`)
	})

	log.Println("Listening on", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

var (
	tmpl      *template.Template
	tmplMu    sync.Mutex
	tmplFuncs = template.FuncMap{
		"popularQueries": func() []string { return popularQueries },
		"queryURL": func(q string) string {
			return "/?" + url.Values{"q": []string{q}}.Encode()
		},
		"sgAssetURL": func(path string) string {
			return sgAssetURL.ResolveReference(&url.URL{Path: path}).String()
		},
		"assetInfix": func() string {
			if *dev {
				return "."
			}
			return ".min."
		},
		"voteLink": func(voteFor, label, class string) template.HTML {
			return template.HTML(fmt.Sprintf(`<a class="%s" target="_blank" href="https://twitter.com/intent/tweet?text=%s&via=srcgraph&url=%s">%s</a>`, class, url.QueryEscape(fmt.Sprintf("I wish #xconf let me search & see examples of %s config files", voteFor)), url.QueryEscape("http://xconf.io"), label))
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
	popularQueries = []string{
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

		TimeoutError bool
		OtherError   bool
	}
	data.Query = strings.TrimSpace(r.FormValue("q"))

	if data.Query != "" {
		deadline := time.Now().Add(*queryTimeout)
		var err error
		data.Results, err = query(data.Query, deadline)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				data.TimeoutError = true
			}
			if err == errQueryTimeout {
				data.TimeoutError = true
			}
			if !data.TimeoutError {
				data.OtherError = true
			}
			log.Printf("Query %s error: %s", data.Query, err)
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

var isDockerfileInstruction = map[string]bool{
	"from": true, "maintainer": true, "run": true, "cmd": true,
	"expose": true, "env": true, "add": true, "copy": true,
	"entrypoint": true, "volume": true, "user": true, "workdir": true,
	"onbuild": true,
}

func query(query string, deadline time.Time) ([]*sourcegraph.Sourcebox, error) {
	if isDockerfileInstruction[strings.ToLower(query)] {
		// HACK: it searches the JSON encoded text, which means the text looks like "\nADD".
		query = "n" + query
	}
	opt := &sourcegraph.UnitListOptions{
		Query:       query,
		UnitType:    "Dockerfile",
		ListOptions: sourcegraph.ListOptions{PerPage: 4},
	}
	units, _, err := sgc.Units.List(opt)
	if err != nil {
		return nil, err
	}
	var sourceboxURLs []string
	for _, u := range units {
		su, err := u.SourceUnit()
		if err != nil {
			return nil, err
		}
		sourceboxURL := sgc.BaseURL.ResolveReference(&url.URL{
			Path: fmt.Sprintf("/%s@%s/.tree/%s/.sourcebox.json", u.Repo, u.CommitID, su.Files[0]),
		})
		sourceboxURLs = append(sourceboxURLs, sourceboxURL.String())
	}
	return getSourceboxes(sourceboxURLs, deadline)
}

func getSourceboxes(urls []string, deadline time.Time) ([]*sourcegraph.Sourcebox, error) {
	getSourcebox := func(url string) (*sourcegraph.Sourcebox, error) {
		resp, err := httpClient.Get(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("http response status %d from %s", resp.StatusCode, url)
		}
		var sb *sourcegraph.Sourcebox
		return sb, json.NewDecoder(resp.Body).Decode(&sb)
	}

	sbs := make([]*sourcegraph.Sourcebox, len(urls))
	errc := make(chan error)
	for i, url := range urls {
		go func(i int, url string) {
			sb, err := getSourcebox(url)
			sbs[i] = sb
			errc <- err
		}(i, url)
	}

	var firstErr error
	timedOut := time.After(deadline.Sub(time.Now()))
	okCount := 0
	for range urls {
		select {
		case err := <-errc:
			if err == nil {
				okCount++
			} else if err != nil && firstErr == nil {
				firstErr = err
			}
		case <-timedOut:
			if firstErr == nil {
				firstErr = errQueryTimeout
			}
			break
		}
	}

	// non-nil sbs
	okSBs := make([]*sourcegraph.Sourcebox, 0, okCount)
	for _, sb := range sbs {
		if sb != nil {
			okSBs = append(okSBs, sb)
		}
	}
	return okSBs, firstErr
}

var errQueryTimeout = errors.New("results timeout")

func newCancelableHTTPMemoryCacheTransport() http.RoundTripper {
	// httpcache doesn't support CancelRequest; wrap it. TODO(sqs):
	// submit a patch to httpcache to fix this.
	t := httpcache.NewMemoryCacheTransport()
	t.Transport = http.DefaultTransport
	return &cancelableHTTPCacheTransport{t}
}

type cancelableHTTPCacheTransport struct{ *httpcache.Transport }

func (t *cancelableHTTPCacheTransport) CancelRequest(req *http.Request) {
	t.Transport.Transport.(*http.Transport).CancelRequest(req)
}
