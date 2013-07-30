package htcat

import (
	"io"
	"sync/atomic"
)

type fragment struct {
	ord      int64
	contents io.ReadCloser
}

type defrag struct {
	// The last successfully written fragment ordinal.
	lastWritten int64

	// The next unallocated fragment ordinal.
	lastAllocated int64

	// The last fragment ordinal to be written.
	lastOrdinal       int64
	lastOrdinalNotify chan int64

	// Fragments being held for future defragmentation.
	future map[int64]*fragment

	// Accepts new fragments ready to be defragmented.
	completeNotify chan *fragment

	// Injects an error to be emitted from WriteTo.
	//
	// Useful if the input stream being defragmented has an
	// unrecoverable problem.
	cancellation error
	cancelNotify chan error

	// Bytes written out so far.
	written int64
}

func newDefrag() *defrag {
	ret := defrag{}
	ret.initDefrag()

	return &ret
}

func (d *defrag) initDefrag() {
	d.future = make(map[int64]*fragment)
	d.completeNotify = make(chan *fragment)
	d.cancelNotify = make(chan error)
	d.lastOrdinalNotify = make(chan int64)
}

func (d *defrag) nextFragment() *fragment {
	atomic.AddInt64(&d.lastAllocated, 1)
	f := fragment{ord: d.lastAllocated}

	return &f
}

func (d *defrag) cancel(err error) {
	d.cancelNotify <- err
}

func (d *defrag) WriteTo(dst io.Writer) (written int64, err error) {
	// Early exit if previously canceled.
	if d.cancellation != nil {
		return d.written, d.cancellation
	}

	for {
		// Exit if all fragments have finished.
		//
		// A fragment ordinal of zero is indeterminate and is
		// specifically excluded from being able to satisfy
		// that criteria.
		if d.lastWritten >= d.lastOrdinal && d.lastOrdinal > 0 {
			break
		}

		// Get fragment, but also check for cancellation.
		var frag *fragment
		select {
		case frag = <-d.completeNotify:
			break
		case d.cancellation = <-d.cancelNotify:
			d.future = nil
			return d.written, d.cancellation
		case d.lastOrdinal = <-d.lastOrdinalNotify:
			continue
		}

		next := d.lastWritten + 1
		if frag.ord == next {
			// This fragment completes the next contiguous
			// swathe of bytes to be written out.
			n, err := d.writeConsecutive(dst, frag)
			d.written += n
			if err != nil {
				return d.written, err
			}
		} else if frag.ord > next {
			// Got a fragment that can't be emitted yet:
			// store it for now.
			d.future[frag.ord] = frag
		} else {
			return d.written, AssertErrf(
				"Unexpected retrograde fragment %v, "+
					"expected at least %v", frag.ord, next)
		}
	}

	return d.written, nil
}

func (d *defrag) setLast(lastOrdinal int64) {
	d.lastOrdinalNotify <- lastOrdinal
}

func (d *defrag) LastAllocated() int64 {
	return atomic.LoadInt64(&d.lastAllocated)
}

func (d *defrag) register(frag *fragment) {
	d.completeNotify <- frag
}

func (d *defrag) writeConsecutive(dst io.Writer, start *fragment) (
	int64, error) {

	// Write out the explicitly passed fragment.
	written, err := io.Copy(dst, start.contents)
	defer start.contents.Close()
	if err != nil {
		return int64(written), err
	}
	d.lastWritten += 1

	// Write as many contiguous bytes as possible.
	for {
		// Check for sent cancellation.
		select {
		case d.cancellation = <-d.cancelNotify:
			d.future = nil
			return 0, d.cancellation
		default:
		}

		next := d.lastWritten + 1
		if frag, ok := d.future[next]; ok {
			// Found a contiguous segment to write.
			delete(d.future, next)
			n, err := io.Copy(dst, frag.contents)
			written += n
			defer frag.contents.Close()
			if err != nil {
				return int64(written), err
			}

			d.lastWritten = next
		} else {
			// No contiguous segment found.
			return int64(written), nil
		}
	}
}
