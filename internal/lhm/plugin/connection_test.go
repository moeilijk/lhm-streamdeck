package plugin

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPollingReusesConnectionWithTrailingWhitespace(t *testing.T) {
	var connections atomic.Int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"Text":"Computer","Children":[]}`)
		w.(http.Flusher).Flush()
		fmt.Fprint(w, strings.Repeat(" ", 32*1024))
	}))
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	srv.Start()
	defer srv.Close()
	s := Service{url: srv.URL, client: srv.Client()}
	for i := 0; i < 4; i++ {
		if err := s.Recv(); err != nil {
			t.Fatal(err)
		}
	}
	if got := connections.Load(); got != 1 {
		t.Fatalf("4 polls opened %d TCP connections, want 1", got)
	}
}
