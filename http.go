package htcat

import (
	"net/http"
	"net/url"
	"sync"
)

type HtCat struct {
	defrag

	u     *url.URL
	cl    *http.Client
	tasks chan *httpFrag

	// Protect httpFragGen with a Mutex.
	httpFragGenMu sync.Mutex
	httpFragGen
}

func New(client *http.Client,
	u *url.URL, parallelism int, chunkSize, totalSize int64) *HtCat {
	cat := HtCat{
		u:  u,
		cl: client,
	}

	cat.initDefrag()
	cat.targetFragSize = chunkSize
	cat.totalSize = totalSize

	for i := 0; i < parallelism; i += 1 {
		go cat.get()
	}

	return &cat
}

func (cat *HtCat) nextFragment() *httpFrag {
	cat.httpFragGenMu.Lock()
	defer cat.httpFragGenMu.Unlock()

	var hf *httpFrag

	if cat.httpFragGen.hasNext() {
		f := cat.defrag.nextFragment()
		hf = cat.httpFragGen.nextFragment(f)
	} else {
		cat.defrag.setLast(cat.defrag.lastAllocated())
	}

	return hf
}

func (cat *HtCat) get() {
	for {
		hf := cat.nextFragment()
		if hf == nil {
			return
		}

		req := http.Request{
			Method:     "GET",
			URL:        cat.u,
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header:     hf.header,
			Body:       nil,
			Host:       cat.u.Host,
		}

		resp, err := cat.cl.Do(&req)
		if err != nil {
			cat.defrag.cancel(err)
		}

		er := newEagerReader(resp.Body, hf.size)
		hf.fragment.contents = er
		cat.register(hf.fragment)
		er.WaitClosed()
	}
}
