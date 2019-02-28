package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/gosuri/uiprogress"
	"github.com/gosuri/uiprogress/util/strutil"
)

const refreshRate = time.Millisecond * 1000

// PassThru struct
type passThru struct {
	r     io.ReadCloser
	total int
	mux   *sync.RWMutex
}

// Downloder provides download service
type downloader struct {
	pt *passThru

	progress *uiprogress.Progress
	bar      *uiprogress.Bar

	done chan bool
}

func newDownloader() *downloader {
	return &downloader{}
}

// Read implements io.Reader
func (pt *passThru) Read(p []byte) (int, error) {
	n, err := pt.r.Read(p)

	if err == nil {
		pt.mux.Lock()
		pt.total += n
		pt.mux.Unlock()
	}

	return n, err
}

// DownloadFile downloads a file.
func DownloadFile(pt io.Reader, filepath string, done chan bool) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, pt)
	if err != nil {
		return err
	}

	done <- true
	return nil
}

func main() {
	// resp, err := http.Get("https://upload.wikimedia.org/wikipedia/commons/d/d6/Wp-w4-big.jpg")
	resp, err := http.Get("http://ipv4.download.thinkbroadband.com/50MB.zip")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	length := int(resp.ContentLength)

	var mux sync.RWMutex

	pt := &passThru{r: resp.Body, mux: &mux}

	done := make(chan bool)

	go DownloadFile(pt, "avatar.zip", done)

	p := uiprogress.New()
	p.SetRefreshInterval(refreshRate)
	p.Start()

	bar := p.AddBar(length).
		AppendCompleted()

	timeStarted := time.Now()
	bar.PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.PadLeft(
			strutil.PrettyTime(time.Since(timeStarted)),
			5,
			' ',
		)
	})

	bar.AppendFunc(func(b *uiprogress.Bar) string {
		mux.Lock()
		defer mux.Unlock()
		return strutil.Resize(humanize.Bytes(uint64(pt.total)), 10)
	})

	for {
		select {
		case <-time.After(refreshRate):
			mux.Lock()
			bar.Set(pt.total)
			mux.Unlock()

		case <-done:
			bar.Set(length)
			p.Stop()
			fmt.Println("done")
			return
		}
	}
}
