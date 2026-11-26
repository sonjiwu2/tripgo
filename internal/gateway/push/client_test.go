package push_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sonjiwu2/tripgo/internal/gateway/push"
)

func TestRequestPositionAccepted(t *testing.T) {
	driver := uuid.NewString()
	tripID := uuid.NewString()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/push" {
			t.Fatalf("path %s", r.URL.Path)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			RequestID   string            `json:"request_id"`
			RecipientID string            `json:"recipient_id"`
			Kind        string            `json:"kind"`
			Data        map[string]string `json:"data"`
		}
		if err = json.Unmarshal(raw, &body); err != nil {
			t.Fatal(err)
		}
		if body.RecipientID != driver || body.Kind != "REQUEST_POSITION" || body.Data["trip_id"] != tripID {
			t.Fatalf("body %+v", body)
		}
		if body.RequestID == "" {
			t.Fatal("empty request id")
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	gw := push.New(srv.URL, time.Second)
	defer gw.Close()
	if err := gw.RequestPosition(context.Background(), driver, tripID); err != nil {
		t.Fatal(err)
	}
}

func TestRequestPositionStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"ok":false}`))
	}))
	defer srv.Close()

	gw := push.New(srv.URL, time.Second)
	defer gw.Close()
	err := gw.RequestPosition(context.Background(), uuid.NewString(), uuid.NewString())
	if err == nil || err.Error() != "push ответил кодом 503" {
		t.Fatalf("err %v", err)
	}
}

func TestRequestPositionTransport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	gw := push.New(url, 200*time.Millisecond)
	defer gw.Close()
	err := gw.RequestPosition(context.Background(), uuid.NewString(), uuid.NewString())
	if err == nil {
		t.Fatal("expected transport error")
	}
}
