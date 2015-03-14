package htcat

import (
	"io"
	"sync/atomic"
)

type writerToCloser interface {
	io.WriterTo
	io.Closer
}

type fragment struct {
	ord      int64
	contents writerToCloser
}

// The Defragmenter is an implementer of the WriterTo interface that
// operates on the principle of handing out fragments, each with an
// ordinal, expecting to receive them once they've been augmented with
// a io.ReadCloser via 'register'.
//
// The registration step can happen out-of-order.
//
// Upon each registration, defrag.WriteTo will determine whether the
// fragment can start being emitted right away or if it must be stored
// until later work is completed to allow the bytes to be written in
// the proper order.
type defrag struct {
	// The last successfully written fragment ordinal.
	lastWritten int64

	// The next unallocated fragment ordinal.
	lastAlloc int64

	// The last fragment ordinal to be written.
	lastOrdinal       int64
	lastOrdinalNotify chan int64

	// Fragments being held for future defragmentation.
	future map[int64]*fragment

	// Accepts new fragments that are pending defragmentation and
	// emission.
	registerNotify chan *fragment

	// Injects an error to be emitted from WriteTo.
	//
	// Useful if the input stream being defragmented has an
	// unrecoverable problem.
	cancellation error
	cancelNotify chan error

	// Bytes written out so far.
	written int64

	// Gets closed when WriteTo is complete and the defragmenter
	// is shutting down.
	done chan struct{}

	// pool of a buffers for use by eagerReader, after the completion
	// of WriteTo() the pool is freed.
	pool *pool
}

func newDefrag() *defrag {
	ret := defrag{}
	ret.initDefrag()

	return &ret
}

func (d *defrag) initDefrag() {
	d.future = make(map[int64]*fragment)
	d.registerNotify = make(chan *fragment)
	d.cancelNotify = make(chan error)
	d.lastOrdinalNotify = make(chan int64)
	d.done = make(chan struct{})
}

// Generate a new fragment.  These are numbered in the order they
// appear in the output stream.
func (d *defrag) nextFragment() *fragment {
	atomic.AddInt64(&d.lastAlloc, 1)
	f := fragment{ord: d.lastAlloc}

	return &f
}

// Inject an error that will surface from defrag.WriteTo, effectively
// canceling the write.
//
// This blocks until it is known that the cancellation has been
// injected, and no more Write calls are thought to be made from
// defrag.WriteTo.
func (d *defrag) cancel(err error) {
	d.cancelNotify <- err
}

// Write the contents of the defragmenter out to the io.Writer dst.
func (d *defrag) WriteTo(dst io.Writer) (written int64, err error) {
	defer close(d.done)
	defer d.pool.Free()

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

		select {
		case frag := <-d.registerNotify:
			// Got fragment.
			//
			// Figure out whether to write it now or not.
			next := d.lastWritten + 1
			if frag.ord == next {
				// This fragment completes the next
				// contiguous swathe of bytes to be
				// written out.
				n, err := d.writeConsecutive(dst, frag)
				d.written += n
				if err != nil {
					return d.written, err
				}
			} else if frag.ord > next {
				// Got a fragment that can't be
				// emitted yet: store it for now.
				d.future[frag.ord] = frag
			} else {
				return d.written, assertErrf(
					"Unexpected retrograde fragment %v, "+
						"expected at least %v",
					frag.ord, next)
			}

		case d.cancellation = <-d.cancelNotify:
			// Cancel and exit immediately with the
			// injected cancellation.
			d.future = nil
			return d.written, d.cancellation

		case d.lastOrdinal = <-d.lastOrdinalNotify:
			// Re-check the exit conditions as a
			// lastOrdinal has been set.
			continue
		}
	}

	return d.written, nil
}

// Set the last work ordinal to be processed once this is known, which
// can allow defrag.WriteTo to terminate.
func (d *defrag) setLast(lastOrdinal int64) {
	select {
	case d.lastOrdinalNotify <- lastOrdinal:
	case <-d.done:
	}
}

// Get the last allocated fragment.
func (d *defrag) lastAllocated() int64 {
	return atomic.LoadInt64(&d.lastAlloc)
}

// Register a new fragment for defragmentation.
func (d *defrag) register(frag *fragment) {
	d.registerNotify <- frag
}

// Write a contiguous swathe of fragments, starting at the work
// ordinal in 'start'.
func (d *defrag) writeConsecutive(dst io.Writer, start *fragment) (int64, error) {
	// Write out the explicitly passed fragment.
	written, err := start.contents.WriteTo(dst)
	if err != nil {
		return int64(written), err
	}

	if err := start.contents.Close(); err != nil {
		return int64(written), err
	}

	d.lastWritten += 1

	// Write as many contiguous bytes as possible.
	for {
		// Check for sent cancellation between each fragment
		// to abort earlier, when possible.
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
			n, err := frag.contents.WriteTo(dst)
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
