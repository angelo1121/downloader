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

var refreshRate = time.Millisecond * 200

type PassThru struct {
	r     io.Reader
	total uint64
	mux   *sync.RWMutex
}

func (pt *PassThru) Read(p []byte) (int, error) {
	n, err := pt.r.Read(p)

	if err == nil {
		pt.mux.Lock()
		pt.total += uint64(n)
		pt.mux.Unlock()
	}

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

	return nil
}

func main() {
	resp, err := http.Get("https://upload.wikimedia.org/wikipedia/commons/d/d6/Wp-w4-big.jpg")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	length := resp.ContentLength

	var mux sync.RWMutex

	pt := &PassThru{r: resp.Body, mux: &mux}

	go func() {
		DownloadFile(pt, "avatar.jpg")
	}()

	p := uiprogress.New()
	p.SetRefreshInterval(refreshRate)
	p.Start()

	bar := p.AddBar(int(resp.ContentLength)).
		AppendCompleted().
		PrependElapsed()

	bar.AppendFunc(func(b *uiprogress.Bar) string {
		mux.Lock()
		defer mux.Unlock()
		return strutil.Resize(humanize.Bytes(pt.total), 10)
	})

	var total int

	for {
		mux.Lock()
		total = int(pt.total)
		mux.Unlock()

		bar.Set(total)

		if total >= int(length) {
			break
		}

		time.Sleep(refreshRate)
	}

	// bar.Set(total)

	p.Stop()
	fmt.Println("done")
}
