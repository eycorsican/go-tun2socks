package session

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"text/tabwriter"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/eycorsican/go-tun2socks/common/log"
	"github.com/eycorsican/go-tun2socks/common/stats"
)

const maxCompletedSessions = 50

type simpleSessionStater struct {
	sessions          sync.Map
	completedSessions []stats.Session
	server            *http.Server
}

func NewSimpleSessionStater() stats.SessionStater {
	return &simpleSessionStater{}
}

func (s *simpleSessionStater) Start() error {
	log.Infof("Start session stater.")
	sessionStatsHandler := func(respw http.ResponseWriter, req *http.Request) {
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

		p := message.NewPrinter(language.English)
		tablePrint := func(w io.Writer, sessions []stats.Session) {
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
		}

		w := tabwriter.NewWriter(respw, 0, 0, 1, ' ', tabwriter.AlignRight|tabwriter.Debug)
		fmt.Fprintf(w, "Active sessions %d\n", len(sessions))
		tablePrint(w, sessions)
		fmt.Fprintf(w, "\n\n")
		fmt.Fprintf(w, "Recently completed sessions %d\n", len(s.completedSessions))
		tablePrint(w, s.completedSessions)
		w.Flush()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/stats/session/plain", sessionStatsHandler)
	server := &http.Server{Addr: "127.0.0.1:6001", Handler: mux}
	go func() {
		s.server.ListenAndServe()
	}()
	s.server = server
	return nil
}

func (s *simpleSessionStater) Stop() error {
	log.Infof("Stop session stater.")
	return s.server.Close()
}

func (s *simpleSessionStater) AddSession(key interface{}, session *stats.Session) {
	s.sessions.Store(key, session)
}

func (s *simpleSessionStater) GetSession(key interface{}) *stats.Session {
	if sess, ok := s.sessions.Load(key); ok {
		return sess.(*stats.Session)
	}
	return nil
}

func (s *simpleSessionStater) RemoveSession(key interface{}) {
	if sess, ok := s.sessions.Load(key); ok {
		s.completedSessions = append(s.completedSessions, *(sess.(*stats.Session)))
		if len(s.completedSessions) > maxCompletedSessions {
			s.completedSessions = s.completedSessions[1:]
		}
	}
	s.sessions.Delete(key)
}
