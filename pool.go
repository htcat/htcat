package htcat

import "sync"

// A pool is a pool of reusable buffers of a set size. Size should be equal to
// the requests fragment size.
type pool struct {
	size int64
	bufs [][]byte
	mu   sync.Mutex
}

// newPool, Returns a new pool with a capicatiy of cap.  Only slices with a
// length of size will be stored in the pool.
func newPool(cap int, size int64) *pool {
	return &pool{
		bufs: make([][]byte, 0, cap),
		size: size,
		mu:   sync.Mutex{},
	}
}

// Put, attempts to put byte slice b into the pool.  If the pool is full or if
// the length of p does not equal the pools size it will be discarded.
func (p *pool) Put(b []byte) {
	p.mu.Lock()
	if int64(len(b)) == p.size && len(p.bufs) < cap(p.bufs) {
		p.bufs = append(p.bufs, b)
	}
	p.mu.Unlock()
}

// Get, returns a byte slice of size n.  If the pool is empty or n is greater
// than the pools buffer size a new byte slice is made.
func (p *pool) Get(n int64) (buf []byte) {
	p.mu.Lock()
	if ln := len(p.bufs); ln != 0 && n <= p.size {
		buf = p.bufs[ln-1][:n]
		p.bufs = p.bufs[:ln-1]
	} else {
		buf = make([]byte, n)
	}
	p.mu.Unlock()
	return buf
}

// Free, sets the pool's buffer to nil so that lingering references do not
// prevent the garbage collector from reclaiming the pools contents.
func (p *pool) Free() {
	if p != nil {
		p.mu.Lock()
		p.bufs = nil
		p.mu.Unlock()
	}
}
