slog
====
Write logs easily in golang.

Usage
----
Common logging
```go
package greet
func hello() {
    slog.Logln(hello, "good morning")
}
```
output:
```
2017/12/01 15:38:44 [greet.hello] good morning
```

Log benchmark to desired stream
```go
package bench
func latencyTest() {
    slog.SetBenchOutput(os.Stdout)  // default stream is os.Stdout
    slog.SetBenchClock(ntped.Now)   // default clock is time.Now()
    slog.Benchln(latencyTest, "hello latency")
}
```
output:
```
2017/12/01 15:42:11 [bench.latencyTest] hello latency
```

Dependencies
----
no external dependency

License
----
MIT
