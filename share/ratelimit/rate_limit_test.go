package ratelimit

import (
	"fmt"
	"testing"
	"time"
)

const threshold = 10

func TestRateLimiter_Acquire(t *testing.T) {
	r := NewRateLimiter(1)
	success := 0
	for {
		r.Acquire()
		fmt.Println(time.Now())

		success++
		if success > threshold {
			break
		}
	}
}

func TestRateLimiter_TryAcquire(t *testing.T) {
	r := NewRateLimiter(1)
	success := 0
	for {
		if r.TryAcquire() {
			fmt.Println(time.Now())
			success++
			if success > threshold {
				break
			}
		} else {
			fmt.Println("acquire fail, sleep 500ms")
			time.Sleep(time.Millisecond * 500)
		}

		if success > threshold {
			break
		}
	}
}

func TestRateLimiter_AcquireWithTimeout(t *testing.T) {
	r := NewRateLimiter(1)
	success := 0
	for {
		if err := r.AcquireWithTimeout(time.Millisecond * 500); err != nil {
			fmt.Println(time.Now())
			success++
			if success > threshold {
				break
			}
		} else {
			fmt.Println("acquire timeout")
		}
		if success > threshold {
			break
		}
	}
}
