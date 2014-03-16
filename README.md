# htcat #

`htcat` is a utility to perform parallel, pipelined execution of a
single HTTP `GET`.  `htcat` is intended for the purpose of
incantations like:

    htcat https://host.net/file.tar.gz | tar -zx

It is tuned (and only really useful) for faster interconnects:

    $ htcat http://test.com/file | pv -a > /dev/null
    [ 109MB/s]

This is on a gigabit network, between an AWS EC2 instance and S3.
This represents 91% use of the theoretical maximum of gigabit (119.2
MiB/s).

## Installation ##

This program depends on Go 1.1 or later.  One can use `go get` to
download and compile it from source:

    $ go get github.com/htcat/htcat/cmd/htcat

## Help and Reporting Bugs ##

For correspondence of all sorts, write to <htcat@googlegroups.com>.
Bugs can be filed at
[htcat's GitHub Issues page](https://github.com/htcat/htcat/issues).

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

On somewhat small files, the situation changes: `htcat` chooses
smaller parts, as to still get some parallelism.

Below are results while performing a 13MB transfer from S3 (Seattle)
to an EC2 instance in Virginia.  Notably, TLS being on or off did not
seem to matter, perhaps in this case it was not a bottleneck.

| Tool   | Time     |
|--------|----------|
| curl   | 5.20s    |
| curl   | 7.75s    |
| curl   | 6.36s    |
| htcat  | 2.69s    |
| htcat  | 2.50s    |
| htcat  | 3.25s    |

Results while performing a transfer of the same 13MB file from S3 to
EC2, but all within Virginia:

| Tool       | TLS | Time     |
|------------|-----|----------|
| curl       | no  | 0.29s    |
| curl       | no  | 0.75s    |
| curl       | no  | 0.44s    |
| htcat      | no  | 0.30s    |
| htcat      | no  | 0.30s    |
| htcat      | no  | 0.48s    |
| curl       | yes | 2.69s    |
| curl       | yes | 2.69s    |
| curl       | yes | 2.62s    |
| htcat      | yes | 1.37s    |
| htcat      | yes | 0.45s    |
| htcat      | yes | 0.59s    |

Results while performing a 4.6MB transfer on a fast (same-region)
link.  This file is small enough that `htcat` disables multi-request
parallelism.  Given that, it's unclear why `htcat` performs markedly
better on the TLS tests than `curl`.

| Tool       | TLS | Time     |
|------------|-----|----------|
| curl       | no  | 0.14s    |
| curl       | no  | 0.13s    |
| curl       | no  | 0.14s    |
| htcat      | no  | 0.23s    |
| htcat      | no  | 0.16s    |
| htcat      | no  | 0.17s    |
| curl       | yes | 0.95s    |
| curl       | yes | 0.97s    |
| curl       | yes | 0.99s    |
| htcat      | yes | 0.38s    |
| htcat      | yes | 0.34s    |
| htcat      | yes | 0.24s    |
