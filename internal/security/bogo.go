package security

import (
	"context"
	"time"

	"project/internal/random"
)

// Bogo is used to use bogo sort to wait time, If timeout, it will interrupt.
type Bogo struct {
	n       int             // random number count
	timeout time.Duration   // wait timeout
	fakeFn  func()          // if compared failed, call this function
	result  map[string]bool // confuse result
	key     string          // key for store result

	ctx    context.Context
	cancel context.CancelFunc
}

// NewBogo is used to create a bogo waiter.
func NewBogo(n int, timeout time.Duration, fakeFn func()) *Bogo {
	if n < 2 {
		n = 2
	}
	if timeout < 1 || timeout > 5*time.Minute {
		timeout = 10 * time.Second
	}
	if fakeFn == nil {
		fakeFn = func() {}
	}
	rand := random.NewRand()
	b := Bogo{
		n:       n,
		timeout: timeout,
		fakeFn:  fakeFn,
		result:  make(map[string]bool),
		key:     rand.String(32 + rand.Int(32)),
	}
	b.ctx, b.cancel = context.WithTimeout(context.Background(), b.timeout)
	return &b
}

// Wait is used to wait bogo sort.
func (bogo *Bogo) Wait() {
	defer bogo.cancel()
	rand := random.NewRand()
	// generate random number
	num := make([]int, bogo.n)
	for i := 0; i < bogo.n; i++ {
		num[i] = rand.Int(100000)
	}
	// confuse result map
	// max = 256 * 64 = 16 KB
	c := 128 + rand.Int(128)
	for i := 0; i < c; i++ {
		l := 32 + rand.Int(32)
		bogo.result[rand.String(l)] = true
	}
swap:
	for {
		// check timeout
		select {
		case <-bogo.ctx.Done():
			return
		default:
		}
		// swap
		for i := 0; i < bogo.n; i++ {
			j := rand.Int(bogo.n)
			num[i], num[j] = num[j], num[i]
		}
		// check is sorted
		for i := 1; i < bogo.n; i++ {
			if num[i-1] > num[i] {
				continue swap
			}
		}
		// set result
		bogo.result[bogo.key] = true
		return
	}
}

// Compare is used to compare the result is correct.
func (bogo *Bogo) Compare() bool {
	if !bogo.result[bogo.key] {
		bogo.fakeFn()
		return false
	}
	return true
}

// Stop is used ot stop wait.
func (bogo *Bogo) Stop() {
	bogo.cancel()
}
