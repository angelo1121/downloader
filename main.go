package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gosuri/uiprogress"
	"github.com/gosuri/uiprogress/util/strutil"
)

type downloadStatus string

const (
	statusPreparing   downloadStatus = "preparing"
	statusReady       downloadStatus = "ready"
	statusDownloading downloadStatus = "downloading"
	statusDone        downloadStatus = "done"
	statusError       downloadStatus = "error"
)

const (
	refreshRate = time.Second
	timeout     = 30 * time.Second
	barLength   = 100
)

type passThru struct {
	r     io.Reader
	total uint64
}

func (pt *passThru) Read(p []byte) (int, error) {
	n, err := pt.r.Read(p)
	pt.total += uint64(n)
	return n, err
}

type downloader struct {
	pt            *passThru
	bar           *uiprogress.Bar
	url           string
	filename      string
	contentLength uint64
	done          chan struct{}
	timeStarted   time.Time
	timeEnded     time.Time
	status        downloadStatus
	error         error
}

func newDownloader(url, filename string, p *uiprogress.Progress) *downloader {
	done := make(chan struct{})
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

	// Download size status
	bar.PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.Resize(fmt.Sprintf("%s/%s", humanize.Bytes(pt.total), humanize.Bytes(d.contentLength)), 15)
	})
	// Download status: preparing, downloading, done,
	bar.PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.Resize(string(d.status), 12)
	})
	// Downloading time in seconds
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
	// Display error message if any
	bar.AppendFunc(func(b *uiprogress.Bar) string {
		if d.error != nil {
			return strutil.Resize(d.error.Error(), 50)
		}

		return ""
	})

	return d
}

func (d *downloader) start() {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", d.url, nil)
	if err != nil {
		d.fail("new request failed", err)
		return
	}

	req.Header.Set("User-Agent", "Mozilla/5.0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		d.fail("sending request failed", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		d.fail("request failed", errors.New(resp.Status))
		return
	}

	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		size, err := strconv.ParseInt(contentLength, 10, 64)
		if err != nil {
			d.fail("parsing content length failed", err)
			return
		}
		d.contentLength = uint64(size)
	} else {
		d.fail("no content-length", errors.New("unable to determine content size before reading"))
		return
	}

	d.pt.r = resp.Body
	d.timeStarted = time.Now()
	d.status = statusDownloading

	go func() {
		if err = d.output(); err != nil {
			d.fail("output failed", err)
		}
	}()

	ticker := time.NewTicker(refreshRate)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err = d.bar.Set(int(float64(d.pt.total) * float64(barLength) / float64(d.contentLength))); err != nil {
				d.fail("refreshing bar failed", err)
				return
			}
		case <-d.done:
			if err = d.bar.Set(barLength); err != nil {
				d.fail("refreshing bar failed", err)
				return
			}
			d.pt.total = d.contentLength
			d.status = statusDone
			d.timeEnded = time.Now()
			return
		case <-ctx.Done():
			d.fail("request timeout", ctx.Err())
			return
		}
	}
}

func (d *downloader) output() error {
	out, err := os.Create(d.filename)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, d.pt)
	if err != nil {
		//return err
		return fmt.Errorf("copy failed: %w", err)
	}

	d.done <- struct{}{}
	return nil
}

func (d *downloader) fail(ctx string, err error) {
	d.error = fmt.Errorf("%s: %w", ctx, err)
	d.status = statusError
}

func main() {
	p := uiprogress.New()
	p.Start()

	var wg sync.WaitGroup

	downloads := []struct {
		url      string
		filename string
	}{
		{"https://freetestdata.com/wp-content/uploads/2022/11/Free_Test_Data_10.5MB_PDF.pdf", "test1.pdf"},
	}

	for _, d := range downloads {
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
