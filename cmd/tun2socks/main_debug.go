// +build debug

// To view all available profiles, open http://localhost:6060/debug/pprof/ in your browser.

package main

import (
	"net/http"
	_ "net/http/pprof"
)

func init() {
	go func() {
		http.ListenAndServe("0.0.0.0:6060", nil)
	}()
}
