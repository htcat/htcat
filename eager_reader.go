package htcat

import (
	"io"
	"sync"
)

type eagerReader struct {
	closeNotify chan struct{}
	rc          io.ReadCloser

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
		rc:          r,
		buf:         make([]byte, bufSz, bufSz),
	}

	er.more = sync.NewCond(&er.bufMu)

	go er.buffer()

	return &er
}

func (er *eagerReader) buffer() {
	for er.lastErr == nil {
		var n int

		er.bufMu.Lock()
		n, er.lastErr = er.rc.Read(er.buf[er.end:])
		er.end += n

		er.more.Broadcast()
		er.bufMu.Unlock()
	}
}

func (er *eagerReader) writeOnce(dst io.Writer) (int64, error) {
	er.bufMu.Lock()
	defer er.bufMu.Unlock()

	if er.begin == er.end && er.end == len(er.buf) {
		return 0, er.lastErr
	}

	// Empty buffer without error: wait for another read
	// to show up.
	for er.end-er.begin == 0 && er.lastErr == nil {
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
	err := er.rc.Close()
	er.closeNotify <- struct{}{}
	return err
}

func (er *eagerReader) WaitClosed() {
	<-er.closeNotify
}
