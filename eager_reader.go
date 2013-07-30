package htcat

import (
	"io"
	"sync"
)

type eagerReader struct {
	closeNotify chan struct{}
	io.ReadCloser

	bufMu sync.Mutex
	buf   []byte
	more  *sync.Cond
	begin int
	end   int

	lastErr error
}

func newEagerReader(r io.ReadCloser, bufSz int64) *eagerReader {
	er := eagerReader{
		closeNotify: make(chan struct{}),
		ReadCloser:  r,
		buf:         make([]byte, bufSz, bufSz),
	}

	er.more = sync.NewCond(&er.bufMu)

	go er.buffer()

	return &er
}

func (er *eagerReader) buffer() {
	for {
		var n int

		er.bufMu.Lock()
		n, er.lastErr = er.ReadCloser.Read(er.buf[er.end:])
		er.end += n

		if er.lastErr != nil {
			er.more.Broadcast()
			er.bufMu.Unlock()
			return
		}

		er.more.Broadcast()
		er.bufMu.Unlock()
	}
}

func (er *eagerReader) Read(p []byte) (int, error) {
	er.bufMu.Lock()
	defer er.bufMu.Unlock()

	// Empty buffer without error: wait for another read
	// to show up.
	for er.end-er.begin == 0 && er.lastErr == nil {
		er.more.Wait()
	}

	// More buffer than requested read: no need to report
	// errors yet.
	if er.end-er.begin > len(p) {
		copy(p, er.buf[er.begin:er.begin+len(p)])
		er.begin += len(p)
		return len(p), nil
	}

	// More read than data buffered: return err that
	// terminated the stream along with any trailing
	// bytes.
	copy(p, er.buf[er.begin:er.end])
	n := er.end - er.begin
	er.begin = er.end
	return n, er.lastErr
}

func (er *eagerReader) Close() error {
	err := er.ReadCloser.Close()
	er.closeNotify <- struct{}{}
	return err
}

func (er *eagerReader) WaitClosed() {
	<-er.closeNotify
}
