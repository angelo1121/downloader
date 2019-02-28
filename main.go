package main

import (
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

type downloader struct {
	pt *passThru

	done chan bool

	bar *uiprogress.Bar

	contentLength int
}

func new(url string, p *uiprogress.Progress) *downloader {
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}

	length := int(resp.ContentLength)

	var mux sync.RWMutex

	pt := &passThru{r: resp.Body, mux: &mux}

	done := make(chan bool)

	bar := p.AddBar(length).AppendCompleted()

	bar.PrependFunc(func(b *uiprogress.Bar) string {
		mux.Lock()
		defer mux.Unlock()
		return strutil.Resize(humanize.Bytes(uint64(pt.total)), 10)
	})

	return &downloader{
		pt:            pt,
		done:          done,
		bar:           bar,
		contentLength: length,
	}
}

func (d *downloader) start() {
	go func() {
		if err := DownloadFile(d.pt, "avatar.zip", d.done); err != nil {
			panic(err)
		}
	}()

	for {
		select {
		case <-time.After(refreshRate):
			d.pt.mux.Lock()
			d.bar.Set(d.pt.total)
			d.pt.mux.Unlock()

		case <-d.done:
			d.bar.Set(d.contentLength)
			return
		}
	}
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

func (pt *passThru) Close() error {
	return pt.r.Close()
}

// DownloadFile downloads a file.
func DownloadFile(pt io.ReadCloser, filepath string, done chan bool) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, pt)
	if err != nil {
		return err
	}

	pt.Close()

	done <- true
	return nil
}

func main() {
	// resp, err := http.Get("https://upload.wikimedia.org/wikipedia/commons/d/d6/Wp-w4-big.jpg")
	// resp, err := http.Get("http://ipv4.download.thinkbroadband.com/10MB.zip")

	p := uiprogress.New()
	p.SetRefreshInterval(refreshRate)
	p.Start()

	var wg sync.WaitGroup

	d1 := new("http://ipv4.download.thinkbroadband.com/10MB.zip", p)
	wg.Add(1)
	go func() {
		defer wg.Done()
		d1.start()
	}()

	d2 := new("http://ipv4.download.thinkbroadband.com/5MB.zip", p)
	wg.Add(1)
	go func() {
		defer wg.Done()
		d2.start()
	}()

	wg.Wait()

	p.Stop()
}
