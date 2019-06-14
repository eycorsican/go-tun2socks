package stats

import (
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/eycorsican/go-tun2socks/common/log"
)

type Session struct {
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

type simpleSessionStater struct {
	sessions sync.Map
}

func NewSimpleSessionStater() SessionStater {
	s := &simpleSessionStater{}
	go s.listenEvent()
	return s
}

func (s *simpleSessionStater) AddSession(key interface{}, session *Session) {
	s.sessions.Store(key, session)
}

func (s *simpleSessionStater) RemoveSession(key interface{}) {
	s.sessions.Delete(key)
}

func (s *simpleSessionStater) listenEvent() {
	// FIXME stop
	for {
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, syscall.SIGUSR1)
		<-osSignals
		s.printSessions()
	}
}

func (s *simpleSessionStater) printSessions() {
	total := 0
	printSession := func(key, value interface{}) bool {
		sess := value.(*Session)
		log.Infof("%v %v %v <-> %v %v %v",
			sess.Network,
			time.Now().Sub(sess.SessionStart),
			sess.LocalAddr,
			sess.RemoteAddr,
			atomic.LoadInt64(&sess.UploadBytes),
			atomic.LoadInt64(&sess.DownloadBytes),
		)
		total += 1
		return true
	}
	log.Infof("")
	s.sessions.Range(printSession)
	log.Infof("total %v", total)
	log.Infof("")
}
