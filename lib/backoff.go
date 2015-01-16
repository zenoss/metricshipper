package metricshipper

import (
	"math"
	"sync"
	"time"

	"github.com/zenoss/glog"
)

type Backoff struct {
	window      time.Duration
	max         int
	collisions  float64
	concurrency float64
	sync.Mutex
}

func NewBackoff(window, max, concurrency int) *Backoff {
	return &Backoff{
		window:      time.Duration(window) * time.Second,
		max:         max,
		concurrency: float64(concurrency),
	}
}

func (b *Backoff) Collision() {
	b.Lock()
	defer b.Unlock()
	if b.collisions >= float64(b.max) {
		return
	}
	b.collisions += 1
	go func() {
		<-time.After(b.window)
		b.Lock()
		defer b.Unlock()
		b.collisions -= 1
	}()
}

func (b *Backoff) Wait() {
	if b.collisions == 0 {
		return
	}
	interval := time.Duration((math.Pow(2, b.collisions/b.concurrency) - 1) / 2 * 1000)
	glog.V(2).Infof("Waiting %dms before sending next batch", interval)
	<-time.After(interval * time.Millisecond)
}
