package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Test-only Unix-socket proxy, reachable only on an isolated Docker network.
// It delays start/removal/listing for restore containers, never publishes a
// host port and never logs request bodies, credentials or SQL.
func main() {
	if len(os.Args) > 1 && os.Args[1] == "arm" {
		resp, err := http.Post("http://127.0.0.1:2375/__test/arm?seconds=35", "application/json", strings.NewReader("{}"))
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			os.Exit(1)
		}
		fmt.Println("armed")
		return
	}
	target, _ := url.Parse("http://docker")
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", "/var/run/docker.sock")
	}}
	proxy.FlushInterval = -1
	var mutex sync.Mutex
	armed := false
	seconds := 35
	var until time.Time
	var activated time.Time
	blocked := 0
	startPath := regexp.MustCompile(`^/(?:v[0-9.]+/)?containers/restivra-[a-f0-9]{32}/start$`)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		if r.URL.Path == "/__test/arm" {
			n, e := strconv.Atoi(r.URL.Query().Get("seconds"))
			if e != nil || n < 1 || n > 120 {
				mutex.Unlock()
				http.Error(w, "invalid duration", 400)
				return
			}
			armed = true
			seconds = n
			until = time.Time{}
			activated = time.Time{}
			blocked = 0
			mutex.Unlock()
			w.WriteHeader(200)
			return
		}
		if r.URL.Path == "/__test/state" {
			data := map[string]any{"armed": armed, "activated_at": activated, "until": until, "blocked_requests": blocked}
			mutex.Unlock()
			json.NewEncoder(w).Encode(data)
			return
		}
		if armed && r.Method == "POST" && startPath.MatchString(r.URL.Path) {
			armed = false
			activated = time.Now().UTC()
			until = activated.Add(time.Duration(seconds) * time.Second)
			fmt.Printf("fault activated duration=%ds\n", seconds)
		}
		delay := time.Until(until)
		match := startPath.MatchString(r.URL.Path) || (r.Method == "DELETE" && strings.Contains(r.URL.Path, "/containers/restivra-")) || strings.HasSuffix(r.URL.Path, "/containers/json")
		wait := delay > 0 && match
		if wait {
			blocked++
		}
		mutex.Unlock()
		if wait {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-r.Context().Done():
				return
			}
		}
		proxy.ServeHTTP(w, r)
	})
	server := http.Server{Addr: ":2375", Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}
