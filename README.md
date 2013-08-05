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

On very small files, some of the start-up costs can overwhelm any
benefit.  The following are from a ~800KB file on a CDN:

| Tool       | TLS | Time     |
|------------|-----|----------|
| curl       | yes | 0.07s    |
| curl       | yes | 0.05     |
| curl       | yes | 0.05s    |
| htcat      | yes | 0.16s    |
| htcat      | yes | 0.16s    |
| htcat      | yes | 0.16s    |
