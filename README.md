# htcat #

`htcat` is a utility to perform parallel, pipelined execution of a
single HTTP `GET`.  `htcat` is intended for the purpose of
incantations like:

    htcat https://host.net/file.tar.gz | tar -zx

It is tuned (and only really useful) for faster interconnects:

    $ htcat http://test.com/file | pv -a > /dev/null
    [ 109MB/s]

This is on a Gigabit network, between an AWS EC2 instance and S3.
This represents 91% use of the theoretical maximum.

## Installation ##

This program depends on a Go 1.1 installation.  One can use a remote
`go get` and then `go install` to compile it from source:

    $ go get github.com/htcat/htcat/cmd/htcat
    $ go install github.com/htcat/htcat/cmd/htcat

## Help and Reporting Bugs ##

For correspondence of all sorts, write to <htcat@googlegroups.com>.
Bugs can be filed at
[htcat's Github Issues page](https://github.com/htcat/htcat/issues).

## Approach ##

`htcat` works by determining the size of the `Content-Length` of the
URL passed, and then partitioning the work into a series of `GET`s
that use the `Range` header in the request, with the notable exception
of the first issued `GET`, which has no `Range` header and is used to
both start the transfer and attempt to determine the size of the URL.

Unlike most programs that do similar `Range`-based splitting, the
requests that are performed in parallel are limited to some bytes
ahead of the data emitted so far instead of splitting the entire byte
stream evenly.  The purpose of this is to emit those bytes as soon as
reasonably possible, so that pipelined execution of another tool can,
too, proceed in parallel.

These requests may complete slightly out of order, and are held in
reserve until contiguous bytes can be emitted by a defragmentation
routine, that catenates together the complete, consecutive payloads in
memory for emission.

Tweaking the number of simultaneous transfers and the size of each
`GET` makes a trade-off between latency to fill the output pipeline,
memory usage, and churn in requests and connections and incurring
their associated start-up costs.

If `htcat`'s peer on the server side processes `Range` requests more
slowly than regular `GET` without a `Range` header, then, `htcat`'s
performance can suffer relative to a simpler, single-stream `GET`.

## Numbers ##

These are measurements falling well short of real benchmarks that are
intended to give a rough sense of the performance improvements that
may be useful to you.  These were taken via an AWS EC2 instance
connecting to S3, and there is definitely some variation in runs,
sometimes very significant, especially at the higher speeds.

|Tool       | TLS | Rate     |
|-----------|-----|----------|
|htcat      | no  | 109 MB/s |
|curl       | no  | 36 MB/s  |
|aria2c -x5 | no  | 113 MB/s |
|htcat      | yes | 59 MB/s  |
|curl       | yes | 5 MB/s   |
|aria2c -x5 | yes | 17 MB/s  |

On small files (~10MB), the situation changes: `htcat` chooses smaller
parts, as to still get some parallelism.  The result can still shave
off some seconds:

| Tool       | TLS | Time     |
|------------|-----|----------|
| curl       | yes | 5.20s    |
| curl       | yes | 5.75s    |
| curl       | yes | 12.77s   |
| htcat      | yes | 7.25s    |
| htcat      | yes | 2.90s    |
| htcat      | yes | 2.88s    |
