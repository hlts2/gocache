# gcache

## Requirement
Go (>= 1.8)

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
  
  value1  = "value_1"
)

cache := gocache.New()

// default expire is 50 Seconds
ok := cache.Set(key1, val) // true

v, ok := cache.Get(key1)

```

## Benchmarks

## Author
[hlts2](https://github.com/hlts2)

## LICENSE
gcache released under MIT license, refer [LICENSE](https://github.com/hlts2/gcache/blob/master/LICENSE) file.
