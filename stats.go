package log

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"gopkg.in/alexcesaro/statsd.v2"
)

type ctxKeyStatsd string

const (
	StatsDKey     ctxKeyStatsd = "statsd"
	NoStatsLogKey ctxKeyStatsd = "nostatslog"
)

var noopStatsD, _ = statsd.New(
	statsd.Mute(true),
	statsd.TagsFormat(statsd.InfluxDB),
)

func StatsDMiddleWare(addr string, prefix string, appName string, tags ...string) func(http.Handler) http.Handler {
	t := append([]string{"app", appName}, tags...)
	c, _ := statsd.New(
		statsd.Address(addr),
		statsd.Mute(addr == ""),
		statsd.TagsFormat(statsd.InfluxDB),
		statsd.Tags(t...),
		statsd.Prefix(prefix),
		statsd.ErrorHandler(func(err error) {
			fmt.Print(err)
		}),
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx = context.WithValue(ctx, StatsDKey, c)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	http.Hijacker
	status int
	r      *http.Request
}

func (rec *statusRecorder) Status() int {
	return rec.status
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
}

func (rec *statusRecorder) Request() *http.Request {
	return rec.r
}

func RequestStatsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var c *statsd.Client
		if f, ok := r.Context().Value(NoStatsLogKey).(bool); ok && f {
			c = noopStatsD
		} else {
			c, ok = r.Context().Value(StatsDKey).(*statsd.Client)
			if !ok {
				c = noopStatsD
			}
		}
		c.Count("http.requests", 1)
		rs := time.Now()
		var hj http.Hijacker
		if _hj, ok := w.(http.Hijacker); ok {
			hj = _hj
		}
		rec := &statusRecorder{w, hj, 200, r}
		next.ServeHTTP(rec, r)
		tt := time.Now().Sub(rs).Seconds() * 1000
		c.Timing("http.latency", tt)
		if rec.status >= 200 && rec.status < 300 {
			c.Count("http.status_200", 1)
		} else if rec.status >= 400 && rec.status < 500 {
			c.Count("http.status_400", 1)
		} else if rec.status >= 500 {
			c.Count("http.status_500", 1)
		}
	})
}

func Stats(r *http.Request) *statsd.Client {
	if s, ok := r.Context().Value(StatsDKey).(*statsd.Client); ok {
		return s
	} else {
		return noopStatsD
	}
}
