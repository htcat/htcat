package htcat

import (
	"reflect"
	"runtime"
	"testing"
)

func TestPoolCap(t *testing.T) {
	const (
		capacity = 5
		size     = 5
	)
	p := newPool(capacity, size)
	if cap(p.bufs) != capacity {
		t.Errorf("expected buffer capacity of %d, got %d", capacity, cap(p.bufs))
	}
	for i := 0; i < capacity+1; i++ {
		p.Put(make([]byte, size))
	}
	if cap(p.bufs) != capacity {
		t.Errorf("expected buffer capacity of %d, got %d", capacity, cap(p.bufs))
	}
	if len(p.bufs) != capacity {
		t.Errorf("expected buffer size of %d, got %d", capacity, len(p.bufs))
	}
}

func TestPoolPut(t *testing.T) {
	const (
		capacity = 5
		size     = 5
	)
	p := newPool(capacity, size)
	bb := [][]byte{
		make([]byte, size),
		make([]byte, size+1),
		make([]byte, size-1),
	}
	for _, b := range bb {
		p.Put(b)
	}
	if len(p.bufs) != 1 {
		t.Errorf("expected buffer size of %d, got %d", 1, len(p.bufs))
	}
}

func TestPoolGet(t *testing.T) {
	const (
		capacity = 10
		size     = 5
	)
	p := newPool(capacity, size)
	bb := [][]byte{
		make([]byte, size),
		make([]byte, size),
		make([]byte, size),
	}
	ps := make(map[uintptr]bool, len(bb))
	for i := 0; i < len(bb); i++ {
		ps[reflect.ValueOf(bb[i]).Pointer()] = true
		p.Put(bb[i])
	}

	var b []byte

	// Test that buffers are returned from the pool.
	for i := len(bb) - 1; i > 0; i-- {
		b = p.Get(size)
		if !ps[reflect.ValueOf(b).Pointer()] {
			t.Error("pool failed to return a buffer from the pool")
		}
	}

	// Test that requests for smaller buffers return a buffer from the pool.
	b = p.Get(size - 1)
	if !ps[reflect.ValueOf(b).Pointer()] {
		t.Error("pool failed to return a smaller buffer from the pool")
	}
	if len(b) != size-1 {
		t.Error("invalid buffer size")
	}

	// Test that a new slice is made when the buffer is empty.
	b = p.Get(size)
	if ps[reflect.ValueOf(b).Pointer()] {
		t.Error("pool returned a buffer that is currently in use")
	}
	b = p.Get(size + 1)
	if len(b) != size+1 {
		t.Error("invalid buffer size")
	}
	b = p.Get(size - 1)
	if len(b) != size-1 {
		t.Error("invalid buffer size")
	}
}

// Test that the GC collects the pools buffer after a call to free.
func TestPoolFree(t *testing.T) {
	const (
		capacity = 10
		size     = mB * 10
	)
	var m [2]runtime.MemStats
	p := newPool(capacity, size)
	for i := 0; i < capacity; i++ {
		p.Put(make([]byte, size))
	}
	runtime.ReadMemStats(&m[0])
	p.Free()
	if p.bufs != nil {
		t.Error("failed to free buffer pool")
	}
	runtime.GC()
	runtime.ReadMemStats(&m[1])
	if m[1].Alloc >= m[0].Alloc {
		t.Error("failed to free buffer")
	}
}
