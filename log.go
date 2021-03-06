package log

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/sirupsen/logrus"
)

type rusLogger struct {
	Logger *logrus.Logger
}

func NewLogger(logger *logrus.Logger) func(next http.Handler) http.Handler {
	if logger == nil {
		// kind of a hack
		logger = logrus.WithError(nil).Logger
	}
	return middleware.RequestLogger(&rusLogger{logger})
}

func (l *rusLogger) NewLogEntry(r *http.Request) middleware.LogEntry {
	entry := &rusLoggerEntry{Logger: logrus.NewEntry(l.Logger)}
	logFields := logrus.Fields{}

	if reqID := middleware.GetReqID(r.Context()); reqID != "" {
		logFields["req_id"] = reqID
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	logFields["scheme"] = scheme
	logFields["proto"] = r.Proto
	logFields["method"] = r.Method

	logFields["remote_addr"] = r.RemoteAddr
	// logFields["user_agent"] = r.UserAgent()

	logFields["uri"] = fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)

	entry.Logger = entry.Logger.WithFields(logFields)
	entry.Level = logrus.InfoLevel

	// entry.Logger.Infoln("request started")

	return entry
}

type rusLoggerEntry struct {
	Logger logrus.FieldLogger
	Level  logrus.Level
}

func (l *rusLoggerEntry) Write(status, bytes int, elapsed time.Duration) {
	l.Logger = l.Logger.WithFields(logrus.Fields{
		"status":     status,
		"length":     bytes,
		"elapsed_ms": float64(elapsed.Nanoseconds()) / 1000000.0,
	})

	if status >= 500 {
		l.Logger.Errorln("request completed with error")
	} else {
		switch l.Level {
		case logrus.PanicLevel:
			l.Logger.Panicln("request completed")
		case logrus.FatalLevel:
			l.Logger.Fatalln("request completed")
		case logrus.ErrorLevel:
			l.Logger.Errorln("request completed")
		case logrus.WarnLevel:
			l.Logger.Warnln("request completed")
		case logrus.InfoLevel:
			l.Logger.Infoln("request completed")
		case logrus.DebugLevel:
			l.Logger.Debugln("request completed")
		}
	}
}

func (l *rusLoggerEntry) Panic(v interface{}, stack []byte) {
	l.Logger = l.Logger.WithFields(logrus.Fields{
		"stack": string(stack),
		"panic": fmt.Sprintf("%+v", v),
	})
	fmt.Fprintf(os.Stderr, "Panic: %+v\n", v)
	os.Stderr.Write(stack)
}

// Helper methods used by the application to get the request-scoped
// logger entry and set additional fields between handlers.
//
// This is a useful pattern to use to set state on the entry as it
// passes through the handler chain, which at any point can be logged
// with a call to .Print(), .Info(), etc.

func GetLog(r *http.Request) logrus.FieldLogger {
	entry := middleware.GetLogEntry(r).(*rusLoggerEntry)
	return entry.Logger
}

func LogEntrySetLevel(r *http.Request, level logrus.Level) {
	if entry, ok := r.Context().Value(middleware.LogEntryCtxKey).(*rusLoggerEntry); ok {
		entry.Level = level
	}
}

func LogEntrySetField(r *http.Request, key string, value interface{}) {
	if entry, ok := r.Context().Value(middleware.LogEntryCtxKey).(*rusLoggerEntry); ok {
		entry.Logger = entry.Logger.WithField(key, value)
	}
}

func LogEntrySetFields(r *http.Request, fields map[string]interface{}) {
	if entry, ok := r.Context().Value(middleware.LogEntryCtxKey).(*rusLoggerEntry); ok {
		entry.Logger = entry.Logger.WithFields(fields)
	}
}
