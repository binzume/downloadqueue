package downloader

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/74.0.3729.169 Safari/537.36"

var CustomFetchers []Fetcher
var DefaultFetcher = &simpleFetcher{"http", defaultDownload}

type DownloadQueue struct {
	scheduler *TaskScheduler
	client    *http.Client
	outDir    string
	fetchers  []Fetcher
}

func NewDownloadQueue(dir string, parallels int) *DownloadQueue {
	jar, _ := cookiejar.New(nil)
	return &DownloadQueue{
		scheduler: NewTaskScheduler(parallels, 100, true),
		client:    &http.Client{Jar: jar, Transport: &httpTransport{}},
		outDir:    dir,
		fetchers:  append(CustomFetchers, DefaultFetcher),
	}
}

func (d *DownloadQueue) Enqueue(target string) *TaskState {
	return d.scheduler.TryPostFunc(func() {
		err := d.fetchURL(target)
		if err != nil {
			log.Print(target, " ERROR:", err)
		} else {
			log.Print(target, " OK")
		}
	}, target)
}

func (d *DownloadQueue) fetchURL(target string) error {
	for _, fetcher := range d.fetchers {
		if fetcher.Match(target) {
			return fetcher.Fetch(d, target)
		}
	}
	return fmt.Errorf("invalid url: %s", target)
}

type httpTransport struct{}

func (t *httpTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", UserAgent)
	return http.DefaultTransport.RoundTrip(req)
}

type Fetcher interface {
	Match(target string) bool
	Fetch(d *DownloadQueue, target string) error
}

type simpleFetcher struct {
	urlPrefix    string
	downloadFunc func(d *DownloadQueue, target string) error
}

func (a *simpleFetcher) Match(target string) bool {
	return strings.HasPrefix(target, a.urlPrefix)
}

func (a *simpleFetcher) Fetch(d *DownloadQueue, target string) error {
	return a.downloadFunc(d, target)
}

func defaultDownload(d *DownloadQueue, target string) error {
	u, err := url.Parse(target)
	if err != nil {
		return err
	}
	fname := path.Base(u.Path)
	if fname == "/" {
		fname = "download"
	}
	outfile := filepath.Join(d.outDir, fname)

	resp, err := d.client.Get(target)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(outfile)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
