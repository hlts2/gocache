# gocache [![Go Report Card](https://goreportcard.com/badge/github.com/hlts2/gocache)](https://goreportcard.com/report/github.com/hlts2/gocache)

gocache is simple ultra fast lock-free cache library written in golang

## Requirement
Go (>= 1.9)

## Installation

```shell
go get github.com/hlts2/gocache
```

## Example

### Basic Example

`Set` is `Set(key string, value interface{})`, so you can set any type of object

```go

var (
  key1 = "key_1"
  key2 = "key_2"
  key3 = "key_3"

  value1  = "value_1"
  value2  = 1234
  value3  = struct{}{}
)

cache := gocache.New()

// default expire is 50 Seconds
ok := cache.Set(key1, value1) // true
ok := cache.Set(key2, value2) // true
ok := cache.Set(key3, value3) // true

// get cached data
v, ok := cache.Get(key1)

v, ok := cache.Get(key2)

v, ok := cache.Get(key3)

```

## Benchmarks

[gocache](https://github.com/hlts2/gocache) vs [go-cache](https://github.com/patrickmn/go-cache) vs [gache](https://github.com/kpango/gache) vs [gcache](https://github.com/bluele/gcache)

The version of golang is `go1.10.3 linux/amd64`
![Bench](https://github.com/hlts2/gocache/blob/master/images/benchmarks.png)

## TODO

- [x] Increase benchmark target
- [ ] add comment

## Author
[hlts2](https://github.com/hlts2)

## LICENSE
gcache released under MIT license, refer [LICENSE](https://github.com/hlts2/gcache/blob/master/LICENSE) file.
