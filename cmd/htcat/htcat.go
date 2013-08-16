package main

import (
	"crypto/tls"
	"github.com/htcat/htcat"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
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

	// On fast links (~= saturating gigabit), parallel execution
	// gives a large speedup.
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Begin the GET.
	htc := htcat.New(&client, u, 5, 20*MB)

	if _, err := htc.WriteTo(os.Stdout); err != nil {
		log.Fatalf("aborting: could not write to output stream: %v",
			err)
	}
}
