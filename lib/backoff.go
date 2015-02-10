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
        maxDelay    float64
        base        float64
	collisions  float64
	sync.Mutex
}

func NewBackoff(window, maxCollisions, maxDelay int) *Backoff {
	return &Backoff{
		window:      time.Duration(window) * time.Second,
		max:         maxCollisions,
                maxDelay:    float64(maxDelay),
                base:        math.Pow(2.0, 1/float64(maxCollisions)),
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
	interval := time.Duration(b.maxDelay * (math.Pow(b.base, b.collisions) - 1))
	glog.V(2).Infof("Waiting %dms before sending next batch", interval)
	<-time.After(interval * time.Millisecond)
}
