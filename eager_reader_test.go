package htcat

import (
	"io"
	"io/ioutil"
	"testing"
)

func TestEagerReaderRead(t *testing.T) {
	reader, writer := io.Pipe()
	payload := []byte("hello")

	er := newEagerReader(ioutil.NopCloser(reader), int64(len(payload)))
	done := make(chan struct{})

	go func() {
		writer.Write(payload)
		writer.Close()
	}()

	var buf []byte
	go func() {
		buf, _ = ioutil.ReadAll(er)
		done <- struct{}{}
	}()

	<-done

	if string(buf) != "hello" {
		t.Fatal()
	}
}
