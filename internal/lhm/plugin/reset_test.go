package plugin

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResetReadingMinMax(t *testing.T) {
	const path = "/gpu-nvidia/0/temperature/0"
	for _, tc := range []struct {
		name, reply                   string
		status                        int
		duplicate, missing, wantError bool
	}{
		{name: "selected sensor", reply: `{"result":"ok"}`, status: 200},
		{name: "server failure", reply: `{"result":"fail","message":"unknown id"}`, status: 200, wantError: true},
		{name: "authentication", reply: `{"result":"ok"}`, status: 401, wantError: true},
		{name: "malformed response", reply: `broken`, status: 200, wantError: true},
		{name: "stale selection", missing: true, wantError: true},
		{name: "duplicate paths", duplicate: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/data.json" {
					sensor := fmt.Sprintf(`{"SensorId":%q,"Text":"GPU temp","Value":"50"}`, path)
					if tc.duplicate {
						sensor += "," + sensor
					}
					fmt.Fprintf(w, `{"Children":[%s]}`, sensor)
					return
				}
				posts++
				if r.Method != "POST" || r.URL.Path != "/Sensor" || r.URL.Query().Get("action") != "ResetMinMax" || r.URL.Query().Get("id") != path {
					t.Errorf("wrong reset target: %s %s", r.Method, r.URL)
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.reply)
			}))
			defer server.Close()
			id := makeReadingID("/gpu-nvidia/0", path)
			if tc.missing {
				id++
			}
			err := ResetReadingMinMax(server.URL+"/data.json", "/gpu-nvidia/0", id)
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v, wantError=%v", err, tc.wantError)
			}
			wantPosts := 1
			if tc.missing || tc.duplicate {
				wantPosts = 0
			}
			if posts != wantPosts {
				t.Fatalf("reset requests %d, want %d", posts, wantPosts)
			}
		})
	}
}
