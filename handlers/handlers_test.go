package handlers

import (
	"testing"
	"time"
)

func TestTimeoutHandler(t *testing.T) {
	timeout := 2 * time.Second
	handler := TimeoutHandler(&timeout)
	now := time.Now()
	err := handler()
	if err != nil {
		t.Fatal("error during timeout handler")
	}
	duration := time.Now().Sub(now)
	if duration < timeout {
		t.Fatal("timeout handler didn't sleep long enough")
	}
}
