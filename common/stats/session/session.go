package session

import (
	"fmt"
	"os"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"text/tabwriter"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/eycorsican/go-tun2socks/common/stats"
)

type simpleSessionStater struct {
	sessions sync.Map
}

func NewSimpleSessionStater() stats.SessionStater {
	s := &simpleSessionStater{}
	go s.listenEvent()
	return s
}

func (s *simpleSessionStater) AddSession(key interface{}, session *stats.Session) {
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
	// Make a snapshot.
	var sessions []stats.Session
	s.sessions.Range(func(key, value interface{}) bool {
		sess := value.(*stats.Session)
		sessions = append(sessions, *sess)
		return true
	})

	// Sort by session start time.
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].SessionStart.Sub(sessions[j].SessionStart) < 0
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight|tabwriter.Debug)
	p := message.NewPrinter(language.English)
	fmt.Fprintf(w, "Process Name\tNetwork\tDuration\tLocal Addr\tRemote Addr\tUpload Bytes\tDownload Bytes\t\n")
	for _, sess := range sessions {
		fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\t%v\t\n",
			sess.ProcessName,
			sess.Network,
			time.Now().Sub(sess.SessionStart),
			sess.LocalAddr,
			sess.RemoteAddr,
			p.Sprintf("%d", atomic.LoadInt64(&sess.UploadBytes)),
			p.Sprintf("%d", atomic.LoadInt64(&sess.DownloadBytes)),
		)
	}
	fmt.Fprintf(w, "total %v\n", len(sessions))
	w.Flush()
}
