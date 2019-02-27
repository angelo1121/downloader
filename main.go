package main

import (
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/dustin/go-humanize"
	"github.com/gosuri/uiprogress"
	"github.com/gosuri/uiprogress/util/strutil"
)

type Downloader struct {
	r     io.Reader
	url   string
	total uint64
	bar   *uiprogress.Bar
	mux   *sync.Mutex
}

func New(url string) *Downloader {
	return &Downloader{url: url, mux: new(sync.Mutex)}
}

func (d *Downloader) Start() error {
	resp, err := http.Get(d.url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	bar := uiprogress.AddBar(int(resp.ContentLength)) // Add a new bar

	bar.AppendCompleted()
	bar.PrependElapsed()
	bar.PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.Resize(humanize.Bytes(d.total), 10)
	})

	d.r = resp.Body
	d.bar = bar

	// defer bar.Set(int(resp.ContentLength))

	return DownloadFile(d, "avatar.jpg")
}

func (d *Downloader) Read(p []byte) (int, error) {
	n, err := d.r.Read(p)

	d.mux.Lock()
	defer d.mux.Unlock()
	d.total += uint64(n)

	if err == nil {
		// fmt.Println("Read", n, "bytes for a total of", humanize.Bytes(d.total))
	}

	d.bar.Set(int(d.total))

	return n, err
}

// DownloadFile downloads a file.
func DownloadFile(r io.Reader, filepath string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, r)
	if err != nil {
		return err
	}

	// fmt.Print("\n")

	return nil
}

func main() {
	uiprogress.Start() // start rendering

	d := New("https://upload.wikimedia.org/wikipedia/commons/d/d6/Wp-w4-big.jpg")

	// go func(d *Downloader) {
	d.Start()
	// }(d)
}
