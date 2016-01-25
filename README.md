# timely

If for any reason maintaining a consistent average response time is more important to you than handling as many requests as possible then `timely` is the middleware for you.

### How does it work?
`timely` limits a maximum number of requests that are allowed to be in progress at the same time. If the average response time goes over the target that number of concurrent request is reduced. If the average response time is below target number of allowed concurrent requests is increased.

All requests over the limit immediately return HTTP 503.

For best results make timely the first middleware in your middleware chain

### Use example

```go
r := chi.NewRouter()

conf := timely.Config{
    AvrRequestTime: 100 * time.Millisecond, // target average response time
	SampleInterval: 0.5 * time.Second,      // adjust max concurrent requests every 0.5 seconds
	InitialQueueDepth: 50,                  // start with a maximum of 50 concurrent requests
}
r.Use(timely.TargetAvrTime(conf))

// register your handlers here

http.ListenAndServe(":80", r)
```
