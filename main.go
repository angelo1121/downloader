package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
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

const (
	refreshRate = time.Second
	barLength   = 100
)

type passThru struct {
	r           io.Reader
	total       uint64
	denominator uint64
}

func (pt *passThru) Read(p []byte) (int, error) {
	n, err := pt.r.Read(p)
	if err == nil {
		pt.total += uint64(n)
	}
	return n, err
}

type downloader struct {
	pt            *passThru
	bar           *uiprogress.Bar
	url           string
	filename      string
	contentLength uint64
	done          chan bool
	timeStarted   time.Time
	timeEnded     time.Time
	status        downloadStatus
}

func newDownloader(url, filename string, p *uiprogress.Progress) *downloader {
	done := make(chan bool)
	bar := p.AddBar(barLength).AppendCompleted()
	bar.Empty = '_'

	pt := &passThru{}
	d := &downloader{
		pt:       pt,
		done:     done,
		bar:      bar,
		url:      url,
		filename: filename,
		status:   statusPreparing,
	}

	bar.PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.Resize(fmt.Sprintf("%s/%s", humanize.Bytes(pt.total), humanize.Bytes(d.contentLength)), 15)
	})
	bar.PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.Resize(string(d.status), 12)
	})
	bar.AppendFunc(func(b *uiprogress.Bar) string {
		switch d.status {
		case statusDownloading:
			return strutil.Resize(strutil.PrettyTime(time.Since(d.timeStarted)), 5)
		case statusDone:
			return strutil.Resize(strutil.PrettyTime(d.timeEnded.Sub(d.timeStarted)), 5)
		default:
			return strutil.Resize("0s", 5)
		}
	})

	return d
}

func (d *downloader) start() {
	client := &http.Client{}
	req, err := http.NewRequest("GET", d.url, nil)
	if err != nil {
		fmt.Println("Request Error:", err)
		return
	}

	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Get Error:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("HTTP Error:", resp.Status)
		return
	}

	contentLength := resp.Header.Get("Content-Length")
	if contentLength != "" {
		size, _ := strconv.ParseInt(contentLength, 10, 64)
		d.contentLength = uint64(size)
		d.pt.denominator = d.contentLength / barLength
	} else {
		fmt.Println("Content-Length header is not set. Unable to determine content size before reading.")
	}

	d.pt.r = resp.Body
	d.timeStarted = time.Now()
	d.status = statusDownloading

	go func() {
		if err := d.output(); err != nil {
			fmt.Println("Download Error:", err)
		}
	}()

	for {
		select {
		case <-time.After(refreshRate):
			d.bar.Set(int(d.pt.total / d.pt.denominator))
		case <-d.done:
			d.bar.Set(barLength)
			d.pt.total = d.contentLength
			d.status = statusDone
			d.timeEnded = time.Now()
			return
		}
	}
}

func (d downloader) output() error {
	out, err := os.Create(d.filename)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, d.pt)
	if err != nil {
		return err
	}

	d.done <- true
	return nil
}

func main() {
	p := uiprogress.New()
	p.Start()

	var wg sync.WaitGroup

	downloaders := []struct {
		url      string
		filename string
	}{
		{"https://freetestdata.com/wp-content/uploads/2022/11/Free_Test_Data_10.5MB_PDF.pdf", "test1.pdf"},
	}

	for _, d := range downloaders {
		wg.Add(1)
		go func(url, filename string) {
			defer wg.Done()
			newDownloader(url, filename, p).start()
		}(d.url, d.filename)
	}

	p.SetRefreshInterval(refreshRate)
	wg.Wait()
	p.Stop()
}
