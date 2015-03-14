package htcat

import (
	"io"
	"sync"
)

type eagerReader struct {
	closeNotify chan struct{}
	rc          io.ReadCloser

	buf   []byte
	more  *sync.Cond
	begin int
	end   int

	lastErr error

	// Pointer to defrag's pool so that eagerReader can release
	// its contents when closed.
	pool *pool
}

func newEagerReader(r io.ReadCloser, size int64, p *pool) *eagerReader {
	var buf []byte
	if p != nil {
		buf = p.Get(size)
	} else {
		buf = make([]byte, size)
	}
	er := eagerReader{
		closeNotify: make(chan struct{}),
		rc:          r,
		buf:         buf,
		pool:        p,
	}

	er.more = sync.NewCond(new(sync.Mutex))

	go er.buffer()

	return &er
}

func (er *eagerReader) buffer() {
	for er.lastErr == nil && er.end != len(er.buf) {
		var n int

		er.more.L.Lock()
		n, er.lastErr = er.rc.Read(er.buf[er.end:])
		er.end += n

		er.more.Broadcast()
		er.more.L.Unlock()
	}
}

func (er *eagerReader) writeOnce(dst io.Writer) (int64, error) {
	// Make one attempt at writing bytes from the buffer to the
	// destination.
	//
	// It may be necessary to wait for more bytes to arrive.
	er.more.L.Lock()
	defer er.more.L.Unlock()

	for er.begin == er.end {
		if er.lastErr != nil {
			return 0, er.lastErr
		}

		if er.begin == len(er.buf) {
			return 0, io.EOF
		}

		er.more.Wait()
	}

	n, err := dst.Write(er.buf[er.begin:er.end])
	er.begin += n
	return int64(n), err
}

func (er *eagerReader) WriteTo(dst io.Writer) (int64, error) {
	var written int64

	for {
		n, err := er.writeOnce(dst)
		written += n
		switch err {
		case io.EOF:
			// Finished.
			//
			// The EOF originates from the Read half of
			// the eagerReader, and it's not desirable
			// emit that to the caller of WriteTo: it's
			// assumed that a nil error and a return means
			// that all bytes have been written.
			return 0, nil
		case nil:
			// More bytes to be written still.
			continue
		default:
			// Error encountered, stop execution.
			return written, err
		}
	}
}

func (er *eagerReader) Close() error {
	er.free()
	err := er.rc.Close()
	er.closeNotify <- struct{}{}
	return err
}

// free, puts the eagerReaders buffer into the buffer pool.
func (er *eagerReader) free() {
	if er.pool != nil {
		er.pool.Put(er.buf)
	}
	er.buf = nil // dereference
}

func (er *eagerReader) WaitClosed() {
	<-er.closeNotify
}
