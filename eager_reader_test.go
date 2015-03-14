package htcat

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"
)

func TestEagerReaderRead(t *testing.T) {
	reader, writer := io.Pipe()
	payload := []byte("hello")

	er := newEagerReader(ioutil.NopCloser(reader), int64(len(payload)), nil)
	done := make(chan struct{})

	go func() {
		writer.Write(payload)
		writer.Close()
	}()

	var buf bytes.Buffer
	go func() {
		er.WriteTo(&buf)
		done <- struct{}{}
	}()

	<-done

	if string(buf.Bytes()) != "hello" {
		t.Fatal()
	}
}
