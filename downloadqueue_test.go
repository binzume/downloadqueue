package downloader

import (
	"os"
	"testing"
)

func TestDownloadQueue_Download(t *testing.T) {
	dl := NewDownloadQueue("./", 1)
	ts := dl.Enqueue("https://binzume.net/index.html")
	<-ts.Done()
	os.Remove("index.html")
}
