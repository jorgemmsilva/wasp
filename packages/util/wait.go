package util

import (
	"testing"
	"time"
)

func WaitUntilTrue(t *testing.T, f func() bool, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for {
		if f() {
			return
		}
		time.Sleep(100 * time.Millisecond)
		if time.Now().After(deadline) {
			t.Fatal("timed out while waiting")
		}
	}
}
