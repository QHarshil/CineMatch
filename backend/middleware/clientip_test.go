package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/harshilc/cinematch-backend/middleware"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name        string
		remoteAddr  string
		forwarded   []string
		trustedHops int
		want        string
	}{
		{name: "no proxy uses the socket address", remoteAddr: "203.0.113.7:51234", want: "203.0.113.7"},
		{name: "headers ignored when no proxy is trusted", remoteAddr: "203.0.113.7:51234", forwarded: []string{"198.51.100.1"}, want: "203.0.113.7"},
		{name: "one trusted hop takes the last entry", remoteAddr: "10.0.0.1:443", forwarded: []string{"198.51.100.9"}, trustedHops: 1, want: "198.51.100.9"},
		{name: "a forged first entry is ignored", remoteAddr: "10.0.0.1:443", forwarded: []string{"1.2.3.4, 198.51.100.9"}, trustedHops: 1, want: "198.51.100.9"},
		{name: "repeated headers are read in order", remoteAddr: "10.0.0.1:443", forwarded: []string{"1.2.3.4", "198.51.100.9"}, trustedHops: 1, want: "198.51.100.9"},
		{name: "two hops skip the load balancer", remoteAddr: "10.0.0.1:443", forwarded: []string{"1.2.3.4, 198.51.100.9, 35.191.0.1"}, trustedHops: 2, want: "198.51.100.9"},
		{name: "too few entries falls back to the socket", remoteAddr: "10.0.0.1:443", forwarded: []string{"198.51.100.9"}, trustedHops: 2, want: "10.0.0.1"},
		{name: "garbage falls back to the socket", remoteAddr: "10.0.0.1:443", forwarded: []string{"not-an-ip"}, trustedHops: 1, want: "10.0.0.1"},
		{name: "IPv6 is normalized", remoteAddr: "[::1]:80", forwarded: []string{"2001:DB8::1"}, trustedHops: 1, want: "2001:db8::1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			handler := middleware.ClientIP(tc.trustedHops)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = r.RemoteAddr
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remoteAddr
			for _, v := range tc.forwarded {
				req.Header.Add("X-Forwarded-For", v)
			}
			handler.ServeHTTP(httptest.NewRecorder(), req)
			if got != tc.want {
				t.Errorf("RemoteAddr = %q, want %q", got, tc.want)
			}
		})
	}
}
