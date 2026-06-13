package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	ContextRequestID = "request_id"
	defaultLogDir    = "/home/workCode/easp/logs"
	maxLogSize       = 50 * 1024 * 1024
	maxBackups       = 5
)

var sensitiveKeys = []string{"authorization", "access_token", "refresh_token", "api_key", "apikey", "password", "secret", "credential", "cookie", "token", "key"}

type rotatingFile struct {
	path string
	file *os.File
	mu   sync.Mutex
}

type coreLogger struct {
	app *rotatingFile
	err *rotatingFile
	out io.Writer
}

var global *coreLogger

func Init(logDir string) error {
	if logDir == "" {
		logDir = defaultLogDir
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}
	app, err := newRotatingFile(filepath.Join(logDir, "easp-server.log"))
	if err != nil {
		return err
	}
	errFile, err := newRotatingFile(filepath.Join(logDir, "easp-error.log"))
	if err != nil {
		return err
	}
	global = &coreLogger{app: app, err: errFile, out: os.Stdout}
	log.SetFlags(0)
	log.SetOutput(global)
	Info("server", "logger initialized", Field("log_dir", logDir))
	return nil
}

func newRotatingFile(path string) (*rotatingFile, error) {
	f := &rotatingFile{path: path}
	return f, f.open()
}

func (f *rotatingFile) open() error {
	file, err := os.OpenFile(f.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	f.file = file
	return nil
}

func (f *rotatingFile) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.file == nil {
		if err := f.open(); err != nil {
			return 0, err
		}
	}
	if stat, err := f.file.Stat(); err == nil && stat.Size()+int64(len(p)) > maxLogSize {
		_ = f.file.Close()
		for i := maxBackups - 1; i >= 1; i-- {
			oldName := fmt.Sprintf("%s.%d", f.path, i)
			newName := fmt.Sprintf("%s.%d", f.path, i+1)
			if _, err := os.Stat(oldName); err == nil {
				_ = os.Rename(oldName, newName)
			}
		}
		_ = os.Rename(f.path, f.path+".1")
		if err := f.open(); err != nil {
			return 0, err
		}
	}
	return f.file.Write(p)
}

func (l *coreLogger) Write(p []byte) (int, error) {
	line := strings.TrimSpace(string(p))
	if line == "" {
		return len(p), nil
	}
	entry := map[string]any{
		"time":    time.Now().Format(time.RFC3339Nano),
		"level":   "info",
		"module":  "stdlog",
		"message": RedactString(line),
	}
	b, _ := json.Marshal(entry)
	b = append(b, '\n')
	_, _ = l.out.Write(b)
	_, _ = l.app.Write(b)
	return len(p), nil
}

type LogField struct {
	Key   string
	Value any
}

func Field(key string, value any) LogField { return LogField{Key: key, Value: value} }

func Info(module, message string, fields ...LogField)  { write("info", module, message, fields...) }
func Warn(module, message string, fields ...LogField)  { write("warn", module, message, fields...) }
func Error(module, message string, fields ...LogField) { write("error", module, message, fields...) }

func write(level, module, message string, fields ...LogField) {
	if global == nil {
		log.Printf("%s %s %s", level, module, message)
		return
	}
	entry := map[string]any{
		"time":    time.Now().Format(time.RFC3339Nano),
		"level":   level,
		"module":  module,
		"message": RedactString(message),
	}
	for _, f := range fields {
		entry[f.Key] = redactValue(f.Key, f.Value)
	}
	b, _ := json.Marshal(entry)
	b = append(b, '\n')
	_, _ = global.out.Write(b)
	_, _ = global.app.Write(b)
	if level == "error" {
		_, _ = global.err.Write(b)
	}
}

func redactValue(key string, value any) any {
	if isSensitiveKey(key) {
		return "[REDACTED]"
	}
	switch v := value.(type) {
	case string:
		return RedactString(v)
	case map[string]any:
		m := make(map[string]any, len(v))
		for k, val := range v {
			m[k] = redactValue(k, val)
		}
		return m
	default:
		return value
	}
}

func isSensitiveKey(key string) bool {
	k := strings.ToLower(key)
	for _, s := range sensitiveKeys {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

func RedactString(s string) string {
	out := s
	patterns := []string{"access_token", "refresh_token", "api_key", "authorization", "password", "secret", "credential", "cookie"}
	lower := strings.ToLower(out)
	for _, p := range patterns {
		idx := strings.Index(lower, p)
		if idx >= 0 {
			return "[REDACTED]"
		}
	}
	return out
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = uuid.NewString()
		}
		c.Set(ContextRequestID, reqID)
		c.Header("X-Request-ID", reqID)
		c.Next()
	}
}

func GetRequestID(c *gin.Context) string {
	if v, ok := c.Get(ContextRequestID); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		c.Next()
		latency := time.Since(start)
		fields := []LogField{
			Field("request_id", GetRequestID(c)),
			Field("method", c.Request.Method),
			Field("path", path),
			Field("query", query),
			Field("status", c.Writer.Status()),
			Field("latency_ms", latency.Milliseconds()),
			Field("client_ip", c.ClientIP()),
			Field("user_agent", c.Request.UserAgent()),
		}
		if tid, ok := c.Get("tenant_id"); ok {
			fields = append(fields, Field("tenant_id", tid))
		}
		if uid, ok := c.Get("user_id"); ok {
			fields = append(fields, Field("user_id", uid))
		}
		if len(c.Errors) > 0 {
			fields = append(fields, Field("errors", c.Errors.String()))
		}
		if c.Writer.Status() >= 500 {
			Error("http", "request completed", fields...)
		} else if c.Writer.Status() >= 400 {
			Warn("http", "request completed", fields...)
		} else {
			Info("http", "request completed", fields...)
		}
	}
}

func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		Error("panic", "request panic recovered",
			Field("request_id", GetRequestID(c)),
			Field("method", c.Request.Method),
			Field("path", c.Request.URL.Path),
			Field("client_ip", c.ClientIP()),
			Field("panic", fmt.Sprintf("%v", recovered)),
			Field("stack", string(debug.Stack())),
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Internal server error", "request_id": GetRequestID(c)})
	})
}
