package proxy

import (
	"net/http"
	"testing"
)

func TestListenAddrBindsLoopbackOnly(t *testing.T) {
	if got := ListenAddr(9876); got != "127.0.0.1:9876" {
		t.Fatalf("ListenAddr = %q, want loopback-bound 127.0.0.1:9876 (the proxy injects the user's bearer)", got)
	}
}

func TestNewServerSetsReadHeaderTimeout(t *testing.T) {
	srv := newServer("127.0.0.1:9876", http.NewServeMux())
	if srv.ReadHeaderTimeout <= 0 {
		t.Fatalf("ReadHeaderTimeout = %v, want a positive Slowloris guard", srv.ReadHeaderTimeout)
	}
	if srv.Addr != "127.0.0.1:9876" {
		t.Fatalf("Addr = %q, want the passed address", srv.Addr)
	}
}
