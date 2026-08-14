package hrserver

import (
	"context"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestServer_Shutdown_WaitsForInFlightRequestToComplete(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	handlerFinished := false
	requestStarted := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		time.Sleep(150 * time.Millisecond)
		mu.Lock()
		handlerFinished = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	srv := &Server{httpSrv: &http.Server{Handler: handler}}
	go srv.httpSrv.Serve(ln)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := http.Get("http://" + ln.Addr().String() + "/")
		if err != nil {
			t.Error(err)
			return
		}
		resp.Body.Close()
	}()

	<-requestStarted
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown returned an error: %v", err)
	}

	mu.Lock()
	finished := handlerFinished
	mu.Unlock()
	if !finished {
		t.Error("Shutdown returned before the in-flight request's handler finished — this is the exact regression C12 fixes")
	}

	wg.Wait()
}

func TestServer_Shutdown_NilHTTPServerIsANoop(t *testing.T) {
	srv := &Server{}
	if err := srv.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown on a Server with no httpSrv should be a no-op, got: %v", err)
	}
}
