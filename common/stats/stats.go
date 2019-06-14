package stats

type SessionStater interface {
	AddSession(key interface{}, session *Session)
	RemoveSession(key interface{})
}
