package stats

import (
	"sync/atomic"
	"time"
)

type SessionStater interface {
	AddSession(key interface{}, session *Session)
	RemoveSession(key interface{})
}

type Session struct {
	ProcessName   string
	Network       string
	LocalAddr     string
	RemoteAddr    string
	UploadBytes   int64
	DownloadBytes int64
	SessionStart  time.Time
}

func (s *Session) AddUploadBytes(n int64) {
	atomic.AddInt64(&s.UploadBytes, n)
}

func (s *Session) AddDownloadBytes(n int64) {
	atomic.AddInt64(&s.DownloadBytes, n)
}
