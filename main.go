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

type downloadStatus string

const (
	statusPreparing   downloadStatus = "preparing"
	statusReady       downloadStatus = "ready"
	statusDownloading downloadStatus = "downloading"
	statusDone        downloadStatus = "done"
)

const refreshRate = time.Millisecond * 1000
const barLength = 100

type passThru struct {
	r io.Reader

	barTotal uint8

	total       uint64
	denominator uint64

	mux *sync.RWMutex
}

type downloader struct {
	pt *passThru

	bar *uiprogress.Bar

	url      string
	filename string
	status   downloadStatus

	contentLength uint64

	done chan bool

	timeStarted time.Time
	timeEnded   time.Time
}

func newDownloader(url string, filename string, p *uiprogress.Progress) *downloader {
	done := make(chan bool)

	bar := p.AddBar(barLength).AppendCompleted()

	var mux sync.RWMutex
	pt := &passThru{mux: &mux}

	d := &downloader{
		pt:       pt,
		done:     done,
		bar:      bar,
		url:      url,
		filename: filename,
		status:   statusPreparing,
	}

	bar.PrependFunc(func(b *uiprogress.Bar) string {
		mux.Lock()
		defer mux.Unlock()

		out := fmt.Sprintf("%s/%s", humanize.Bytes(pt.total), humanize.Bytes(d.contentLength))

		return strutil.Resize(out, 15)
	})
	bar.PrependFunc(func(b *uiprogress.Bar) string {
		mux.Lock()
		defer mux.Unlock()

		return strutil.Resize(string(d.status), 12)
	})
	bar.AppendFunc(func(b *uiprogress.Bar) string {
		mux.Lock()
		defer mux.Unlock()

		switch d.status {
		case statusDownloading:
			return strutil.Resize(strutil.PrettyTime(time.Since(d.timeStarted)), 5)
		case statusDone:
			return strutil.Resize(strutil.PrettyTime(d.timeEnded.Sub(d.timeStarted)), 5)
		default:
			return strutil.Resize("0s", 5)
		}
	})

	bar.Set(0)

	return d
}

func (d *downloader) start() {
	resp, err := http.Get(d.url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	pt := d.pt

	pt.mux.Lock()
	d.contentLength = uint64(resp.ContentLength)
	pt.mux.Unlock()

	pt.r = resp.Body

	pt.denominator = d.contentLength / barLength

	go func() {
		pt.mux.Lock()
		d.timeStarted = time.Now()
		pt.mux.Unlock()

		d.status = statusDownloading
		if err := DownloadFile(pt, d.filename, d.done); err != nil {
			panic(err)
		}
	}()

	for {
		select {
		case <-time.After(refreshRate):
			pt.mux.Lock()
			d.bar.Set(int(pt.barTotal))
			pt.mux.Unlock()

		case <-d.done:
			d.bar.Set(barLength)

			pt.mux.Lock()
			defer pt.mux.Unlock()

			pt.total = d.contentLength
			d.status = statusDone
			d.timeEnded = time.Now()

			return
		}
	}
}

// Read implements io.Reader
func (pt *passThru) Read(p []byte) (int, error) {
	n, err := pt.r.Read(p)

	if err == nil {
		pt.mux.Lock()
		defer pt.mux.Unlock()

		pt.total += uint64(n)

		barTotal := pt.total / pt.denominator

		if barTotal <= 1 {
			pt.barTotal = 1
		} else {
			pt.barTotal = uint8(barTotal)
		}
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
	p := uiprogress.New()
	p.Start()

	var wg sync.WaitGroup

	d1 := newDownloader("http://ipv4.download.thinkbroadband.com/10MB.zip", "test.zip", p)
	wg.Add(1)
	go func() {
		defer wg.Done()
		d1.start()
	}()

	d2 := newDownloader("http://ipv4.download.thinkbroadband.com/5MB.zip", "test2.zip", p)
	wg.Add(1)
	go func() {
		defer wg.Done()
		d2.start()
	}()

	d3 := newDownloader("http://ipv4.download.thinkbroadband.com/5MB.zip", "test2.zip", p)
	wg.Add(1)
	go func() {
		time.Sleep(time.Second * 5)
		defer wg.Done()
		d3.start()
	}()

	d4 := newDownloader("http://ipv4.download.thinkbroadband.com/5MB.zip", "test2.zip", p)
	wg.Add(1)
	go func() {
		time.Sleep(time.Second * 5)
		defer wg.Done()
		d4.start()
	}()

	p.SetRefreshInterval(refreshRate)

	wg.Wait()

	p.Stop()
}
