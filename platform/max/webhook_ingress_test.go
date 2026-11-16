package max

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chenhg5/cc-connect/core"
)

func TestIncompleteVoiceUpdateIsObservableWithoutGuessingRecipient(t *testing.T) {
	for _, messageField := range []string{"", `,"message":null`} {
		t.Run(messageField, func(t *testing.T) {
			var logs bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
			defer slog.SetDefault(previous)
			var requests, turns atomic.Int32
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer api.Close()
			p := newTestPlatform(t, api.URL)
			p.chatOfUser.Store("99", "555")
			p.handler = func(core.Platform, *core.Message) { turns.Add(1) }
			const privateValue = "DO-NOT-LOG-UNKNOWN-UPDATE-CONTENT"
			var update maxUpdate
			if err := json.Unmarshal([]byte(`{"update_type":"message_created","timestamp":1794404075604,"user_locale":"ru","unknown":"`+privateValue+`"`+messageField+`}`), &update); err != nil {
				t.Fatal(err)
			}
			p.handleUpdate(context.Background(), &update)
			if requests.Load() != 0 || turns.Load() != 0 {
				t.Fatal("incomplete update guessed a recipient or started an agent turn")
			}
			if !strings.Contains(logs.String(), "reason=missing_message") {
				t.Fatal("missing message was silently discarded")
			}
			if strings.Contains(logs.String(), privateValue) {
				t.Fatal("incomplete update leaked request contents")
			}
		})
	}
}

func TestWebhookRejectsOversizeInsteadOfAcceptingTruncatedJSON(t *testing.T) {
	p := newTestPlatform(t, "http://unused.invalid")
	body := `{"update_type":"unknown"}` + strings.Repeat(" ", 4*1024*1024)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	rec := httptest.NewRecorder()
	p.webhookHandler(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize body: got %d, want 413", rec.Code)
	}
}

func TestWebhookMalformedBodyDoesNotLeakPayloadToLogs(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(previous)
	p := newTestPlatform(t, "http://unused.invalid")
	const sentinel = "DO-NOT-LOG-SENSITIVE-VOICE"
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"token":"`+sentinel+`",`))
	rec := httptest.NewRecorder()
	p.webhookHandler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed body: got %d, want 400", rec.Code)
	}
	if strings.Contains(logs.String(), sentinel) {
		t.Fatal("webhook logged private request contents")
	}
}

func TestWebhookOccupiedPortDoesNotRegisterSubscription(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	var subscriptions atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subscriptions.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer api.Close()
	p := newTestPlatform(t, api.URL)
	p.webhookListen = occupied.Addr().String()
	p.webhookURL = "https://example.invalid/webhook"
	p.webhookPath = "/webhook"
	p.resubscribeInterval = 0
	err = p.startWebhook(context.Background())
	if p.webServer != nil {
		_ = p.webServer.Close()
	}
	if err == nil {
		t.Error("webhook startup succeeded despite occupied port")
	}
	if got := subscriptions.Load(); got != 0 {
		t.Errorf("registered unreachable webhook: %d API calls", got)
	}
}

func TestWebhookSubscriptionFailureClosesListener(t *testing.T) {
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := reservation.Addr().String()
	_ = reservation.Close()
	var subscriptions atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subscriptions.Add(1)
		connection, err := net.DialTimeout("tcp", address, time.Second)
		if err != nil {
			t.Error("subscription started before local listener was ready")
		} else {
			_ = connection.Close()
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer api.Close()
	p := newTestPlatform(t, api.URL)
	p.webhookListen = address
	p.webhookURL = "https://example.invalid/webhook"
	p.webhookPath = "/webhook"
	p.resubscribeInterval = 0
	if err := p.startWebhook(context.Background()); err == nil {
		t.Fatal("expected subscription error")
	}
	if p.webServer != nil {
		_ = p.webServer.Close()
		t.Fatal("failed subscription retained webhook server")
	}
	if subscriptions.Load() != 1 {
		t.Fatal("test did not reach subscription failure")
	}
	reopened, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("failed subscription left port occupied: %v", err)
	}
	_ = reopened.Close()
}
