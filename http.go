package htcat

import (
	"bufio"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
)

const (
	_        = iota
	kB int64 = 1 << (10 * iota)
	mB
	gB
	tB
	pB
	eB
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

func (cat *HtCat) startup(parallelism int) {
	req := http.Request{
		Method:     "GET",
		URL:        cat.u,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Body:       nil,
		Host:       cat.u.Host,
	}

	resp, err := cat.cl.Do(&req)
	if err != nil {
		go cat.defrag.cancel(err)
		return
	}

	l := resp.Header.Get("Content-Length")

	// Some kinds of small or indeterminate-length files will
	// receive no parallelism.  This procedure helps prepare the
	// HtCat value for a one-HTTP-Request GET.
	noParallel := func(wtc writerToCloser) {
		f := cat.defrag.nextFragment()
		cat.defrag.setLast(cat.defrag.lastAllocated())
		f.contents = wtc
		cat.register(f)
	}

	if l == "" {
		// No Content-Length, stream without parallelism nor
		// assumptions about the length of the stream.
		go noParallel(struct {
			io.WriterTo
			io.Closer
		}{
			WriterTo: bufio.NewReader(resp.Body),
			Closer:   resp.Body,
		})
		return
	}

	length, err := strconv.ParseInt(l, 10, 64)
	if err != nil {
		// Invalid integer for Content-Length, defer reporting
		// the error until a WriteTo call is made.
		go cat.defrag.cancel(err)
		return
	}

	// Set up httpFrag generator state.
	cat.totalSize = length
	cat.targetFragSize = length / int64(parallelism)
	if cat.targetFragSize > 20*mB {
		cat.targetFragSize = 20 * mB
	}

	// Very small fragments are probably not worthwhile to start
	// up new requests for, but it in this case it was possible to
	// ascertain the size, so take advantage of that to start
	// reading in the background as eagerly as possible.
	if cat.targetFragSize < 1*mB {
		er := newEagerReader(resp.Body, cat.totalSize)
		go noParallel(er)
		go func() { er.WaitClosed() }()
		return
	}

	// None of the other special short-circuit cases have been
	// triggered, so begin preparation for full-blown parallel
	// GET.  One GET worker is started here to take advantage of
	// the already pending response (which has no determinate
	// length, so it must be limited).
	hf := cat.nextFragment()
	go func() {
		er := newEagerReader(
			struct {
				io.Reader
				io.Closer
			}{
				Reader: io.LimitReader(resp.Body, hf.size),
				Closer: resp.Body,
			},
			hf.size)

		hf.fragment.contents = er
		cat.register(hf.fragment)
		er.WaitClosed()

		// Chain into being a regular worker, having finished
		// the special start-up segment.
		cat.get()
	}()

}

func New(client *http.Client, u *url.URL, parallelism int,
	chunkSize int64) *HtCat {
	cat := HtCat{
		u:  u,
		cl: client,
	}

	cat.initDefrag()
	cat.targetFragSize = chunkSize
	cat.startup(parallelism)

	if cat.totalSize <= 0 {
		return &cat
	}

	// Start background workers.
	//
	// "startup" starts one worker that is specially constructed
	// to deal with the first request, so back off by one to
	// prevent performing with too much parallelism.
	for i := 1; i < parallelism; i += 1 {
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
