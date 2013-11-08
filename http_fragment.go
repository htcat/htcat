package htcat

import (
	"fmt"
	"net/http"
)

type httpFragGen struct {
	curPos    int64
	totalSize int64

	targetFragSize int64
}

type httpFrag struct {
	*fragment
	header http.Header
	size   int64
}

func (fg *httpFragGen) hasNext() bool {
	return fg.curPos < fg.totalSize
}

func (fg *httpFragGen) nextFragment(f *fragment) *httpFrag {
	// Determine fragment size.
	var fragSize int64
	remaining := fg.totalSize - fg.curPos
	if remaining < fg.targetFragSize {
		fragSize = remaining
	} else {
		fragSize = fg.targetFragSize
	}

	hf := httpFrag{
		fragment: f,
		header: http.Header{
			"Range": {fmt.Sprintf("bytes=%d-%d",
				fg.curPos, fg.curPos+fragSize-1)},
		},
		size: fragSize,
	}

	fg.curPos += fragSize

	return &hf
}
