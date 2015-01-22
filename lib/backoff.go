package metricshipper

import (
	"math"
	"sync"
	"time"

	"github.com/zenoss/glog"
)

type Backoff struct {
	maxSteps  int
	maxDelay  float64
	base      float64
	steps     int
	lastDecay int64
	sync.Mutex
}

func NewBackoff(maxSteps, maxDelay int) *Backoff {
	return &Backoff{
		maxSteps:  maxSteps,
		maxDelay:  float64(maxDelay),
		base:      math.Pow(2.0, 1/float64(maxSteps)),
		lastDecay: time.Now().Unix(),
	}
}

func (b *Backoff) decay() {
	b.Lock()
	defer b.Unlock()
	now := time.Now().Unix()
	steps := int(now - b.lastDecay)
	if b.steps > steps {
		b.steps -= steps
	}
	if b.steps < 0 {
		b.steps = 0
	}
	b.lastDecay = now
}

func (b *Backoff) Backoff() {
	b.Lock()
	defer b.Unlock()
	if b.steps >= b.maxSteps {
		return
	}
	b.steps += 1
}

func (b *Backoff) Wait() {
	b.decay()
	if b.steps == 0 {
		return
	}
	interval := time.Duration(b.maxDelay * (math.Pow(b.base, float64(b.steps)) - 1))
	glog.V(2).Infof("Waiting %dms before sending next batch", interval)
	<-time.After(interval * time.Millisecond)
}
