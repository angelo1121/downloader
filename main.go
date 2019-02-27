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

const refreshRate = time.Millisecond * 100

type PassThru struct {
	r     io.Reader
	total int
	mux   *sync.RWMutex
}

func (pt *PassThru) Read(p []byte) (int, error) {
	n, err := pt.r.Read(p)

	if err == nil {
		pt.mux.Lock()
		pt.total += n
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

	go DownloadFile(pt, "avatar.jpg")

	p := uiprogress.New()
	p.SetRefreshInterval(refreshRate)
	p.Start()

	bar := p.AddBar(int(resp.ContentLength)).
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

	var total int

	for {
		mux.Lock()
		total = pt.total
		mux.Unlock()

		bar.Set(total)

		if total >= int(length) {
			break
		}

		time.Sleep(refreshRate)
	}

	p.Stop()
	fmt.Println("done")
}
