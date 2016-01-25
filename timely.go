package timely

import (
	"net/http"
	"sync"
	"time"
)

type Conf struct {
	AvrRequestTime    time.Duration
	SampleInterval    time.Duration
	InitialQueueDepth int
}

func (c *Conf) setDefaults() {
	if c.AvrRequestTime <= 0 {
		c.AvrRequestTime = defaults.AvrRequestTime
	}
	if c.SampleInterval <= 0 {
		c.SampleInterval = defaults.SampleInterval
	}
	if c.InitialQueueDepth <= 0 {
		c.InitialQueueDepth = defaults.InitialQueueDepth
	}
}

type throttler struct {
	queue chan struct{}

	mux       sync.RWMutex
	reqCount  int64
	reqTimeNs int64
}

func (t *throttler) start() bool {
	// To make sure queue is not being replaced
	t.mux.RLock()
	defer t.mux.RUnlock()

	select {
	case t.queue <- struct{}{}:
		return true
	default:
		return false
	}
}

func (t *throttler) end() {
	select {
	case <-t.queue:
	default:
	}
}

func (t *throttler) autoTune(targetAvg time.Duration, sInterval <-chan time.Time) {
	for _ = range sInterval {
		curQueueDep := float64(cap(t.queue))
		targetReqNs := float64(targetAvg.Nanoseconds())

		t.mux.Lock()
		if t.reqCount > 0 {
			avgReqNs := float64(t.reqTimeNs) / float64(t.reqCount)
			newQueueDep := int(targetReqNs / avgReqNs * curQueueDep)

			t.reqCount = 0
			t.reqTimeNs = 0

			if int(curQueueDep) != newQueueDep {
				newQ := make(chan struct{}, newQueueDep)
				close(t.queue)

				for v := range t.queue {
					select {
					case newQ <- v:
					default:
					}
				}

				t.queue = newQ
			}
		}
		t.mux.Unlock()
	}
}

var (
	defaults = Conf{
		AvrRequestTime:    20 * time.Millisecond,
		SampleInterval:    500 * time.Millisecond,
		InitialQueueDepth: 200,
	}
	th throttler
)

var queueBuffer struct {
	throttler chan struct{}
	mutex     sync.RWMutex
}

func TargetAvrTime(c Conf) func(http.Handler) http.Handler {
	c.setDefaults()
	th = throttler{
		queue: make(chan struct{}, c.InitialQueueDepth),
	}
	go th.autoTune(c.AvrRequestTime, time.Tick(c.SampleInterval))

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			if !th.start() {
				http.Error(w, http.StatusText(503), 503)
				return
			}
			// record start time
			sTime := time.Now().UnixNano()

			next.ServeHTTP(w, r)

			th.mux.Lock()
			th.reqCount++
			th.reqTimeNs = th.reqTimeNs + (sTime - time.Now().UnixNano())
			th.end()
			th.mux.Unlock()

		}
		return http.HandlerFunc(fn)
	}
}
