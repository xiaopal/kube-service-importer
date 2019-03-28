package prober

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/xiaopal/kube-service-importer/pkg/fluconf"
)

func testServer(handler func(w http.ResponseWriter, r *http.Request)) (*httptest.Server, *url.URL) {
	ts := httptest.NewServer(http.HandlerFunc(handler))
	u, _ := url.Parse(ts.URL)
	return ts, u
}

func TestProbers(t *testing.T) {
	ts, tsURL := testServer(func(res http.ResponseWriter, req *http.Request) {
		switch req.RequestURI {
		case "/test/200":
			res.WriteHeader(200)
		case "/test/500":
			res.WriteHeader(500)
		default:
			res.WriteHeader(404)
		}
	})
	defer ts.Close()

	t.Run("test-probers", func(t *testing.T) {
		checkConfs := []fluconf.Config{
			{"probe": "http", "host": tsURL.Hostname(), "uri": "/test/200", "port": tsURL.Port(), "timeout": "100ms", "interval": "100ms", "fall": "3", "rise": "2", "result": "OK"},
			{"probe": "http", "host": tsURL.Hostname(), "uri": "/test/500", "port": tsURL.Port(), "timeout": "100ms", "interval": "100ms", "fall": "3", "rise": "2", "result": "FAIL"},
			{"probe": "tcp", "host": tsURL.Hostname(), "port": tsURL.Port(), "timeout": "100ms", "interval": "100ms", "fall": "3", "rise": "2", "result": "OK"},
			{"probe": "tcp", "host": tsURL.Hostname(), "port": "1", "timeout": "100ms", "interval": "100ms", "fall": "3", "rise": "2", "result": "FAIL"},
		}
		for _, conf := range checkConfs {
			key := fmt.Sprintf("%s|%s|%s|%s", conf["probe"], conf["host"], conf["port"], conf["uri"])
			_, stop := StartUpdater(key, LoadSimpleStatusProber(conf))
			defer stop()
		}
		time.Sleep(500 * time.Millisecond)
		for _, conf := range checkConfs {
			key, want := fmt.Sprintf("%s|%s|%s|%s", conf["probe"], conf["host"], conf["port"], conf["uri"]), SimpleStatusResult(conf["result"] == "OK")
			if got, ok := UpdaterStatus(key); !ok || got != want {
				t.Errorf("check=%v, status=%v/%v, want=%v", key, got, ok, want)
			}
		}
	})
}
