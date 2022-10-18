package middleware

import (
	"net"
	"net/http"
	"strings"
)

var (
	xForwardedFor   = http.CanonicalHeaderKey("X-Forwarded-For")
	xForwardedProto = http.CanonicalHeaderKey("X-Forwarded-Proto")
	xRealIP         = http.CanonicalHeaderKey("X-Real-IP")
	forwardLimit    = 5
)

// RealIPMiddleware is an implementation of reverse proxy checks.
// It uses the remote address to find the originating IP, as well as protocol
func RealIPMiddleware(f http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Treat unix socket as 127.0.0.1
		if r.RemoteAddr == "@" {
			r.RemoteAddr = "127.0.0.1:0"
		}

		host, _, err := net.SplitHostPort(r.RemoteAddr)

		if err != nil {
			f.ServeHTTP(w, r)
			return
		}

		netIP := net.ParseIP(host)

		if !netIP.IsLoopback() && !netIP.IsPrivate() {
			f.ServeHTTP(w, r)
			return
		}

		if rip := realIP(r); len(rip) > 0 {
			r.RemoteAddr = net.JoinHostPort(rip, "0")
		}

		if rproto := realProto(r); len(rproto) > 0 {
			r.URL.Scheme = rproto
		}

		f.ServeHTTP(w, r)
	})
}

func realIP(r *http.Request) string {
	var ip string

	if xrip := r.Header.Get(xRealIP); xrip != "" {
		ip = xrip
	} else if xff := r.Header.Get(xForwardedFor); xff != "" {
		p := 0
		for i := forwardLimit; i > 0; i-- {
			if p > 0 {
				xff = xff[:p-2]
			}
			p = strings.LastIndex(xff, ", ")
			if p < 0 {
				p = 0
				break
			} else {
				p += 2
			}
		}

		ip = xff[p:]
	}

	return ip
}

func realProto(r *http.Request) string {
	proto := "http"

	if r.TLS != nil {
		proto = "https"
	}

	if xproto := r.Header.Get(xForwardedProto); xproto != "" {
		proto = xproto
	}

	return proto
}
