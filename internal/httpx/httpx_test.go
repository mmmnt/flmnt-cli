package httpx

import "testing"

func TestClientIsTimeoutBounded(t *testing.T) {
	if Client.Timeout <= 0 {
		t.Fatalf("shared HTTP client timeout = %v, want a positive bound so a hung endpoint can't block a hook", Client.Timeout)
	}
}
