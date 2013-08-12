package main

import (
	"crypto/tls"
	"github.com/htcat/htcat"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
)

const (
	_        = iota
	KB int64 = 1 << (10 * iota)
	MB
	GB
	TB
	PB
	EB
)

func printUsage() {
	log.Printf("usage: %v URL", os.Args[0])
}

func main() {
	if len(os.Args) != 2 {
		printUsage()
		log.Fatalf("aborting: incorrect usage")
	}

	u, err := url.Parse(os.Args[1])
	if err != nil {
		log.Fatalf("aborting: could not parse given URL: %v", err)
	}

	client := *http.DefaultClient

	// Only support HTTP and HTTPS schemes
	switch u.Scheme {
	case "https":
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{},
		}
	case "http":
	default:
		// This error path can be hit with common alphanumeric
		// lexemes like "help", which also parse as URLs,
		// which makes this error message somewhat
		// incomprehensible.  Try to help out a user by
		// printing the usage here.
		printUsage()
		log.Fatalf("aborting: unsupported URL scheme %v", u.Scheme)
	}

	// Run HEAD to determine the length of the payload.
	resp, err := client.Head(os.Args[1])
	if err != nil {
		log.Fatalf("aborting: "+
			"could not run HEAD to determine Content-Length: %v",
			err)
	}
	resp.Body.Close()

	rawLen := resp.Header.Get("Content-Length")
	if rawLen == "" {
		log.Fatalf("aborting: " +
			"no Content-Length response from HTTP HEAD")
	}

	length, err := strconv.ParseInt(rawLen, 10, 64)
	if err != nil {
		log.Fatalf("aborting: could not parse Content-Length: %v")
	}

	if length < 0 {
		log.Fatalf("aborting: host delivered negative Content-Length")
	}

	// On fast links (~= saturating gigabit), parallel execution
	// gives a large speedup.
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Choose the size of each part downloaded in parallel.
	//
	// Done this way to get good throughput with large files, but
	// to also get parallel execution on small GETs as well.
	parallelism := 5

	var partSize int64
	partSize = length / int64(parallelism)

	if partSize > 20*MB {
		partSize = 20 * MB
	}

	// Begin the GET.
	htc := htcat.New(client, u, parallelism, partSize, length)

	if _, err := htc.WriteTo(os.Stdout); err != nil {
		log.Fatalf("aborting: could not write to output stream: %v",
			err)
	}
}
