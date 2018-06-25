package lwip

import (
	"sync"
)

var pool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 1500)
	},
}

func NewBytes() []byte {
	return pool.Get().([]byte)
}

func FreeBytes(b []byte) {
	pool.Put(b)
}
