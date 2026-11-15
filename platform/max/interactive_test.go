package max

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chenhg5/cc-connect/core"
)

func TestMessageMutationRejectsHTTP200Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"code":"chat.denied"}`))
	}))
	defer server.Close()
	p := newTestPlatform(t, server.URL)
	rctx := replyContext{chatID: "555", userID: "99", messageID: "message"}
	if err := p.UpdateMessage(context.Background(), rctx, "edit"); err == nil {
		t.Fatal("failed edit reported as delivered")
	}
	if _, err := p.postMessageID(context.Background(), rctx, &maxSendBody{Text: "send"}); err == nil {
		t.Fatal("failed send reported as delivered")
	}
}

func TestButtonLayoutFollowsMAXTextWidthRecommendations(t *testing.T) {
	var rows [][]maxButton
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Attachments []struct {
				Payload struct{ Buttons [][]maxButton }
			}
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		rows = body.Attachments[0].Payload.Buttons
		_, _ = w.Write([]byte(`{"message":{"body":{"mid":"compact-menu"}}}`))
	}))
	defer server.Close()
	p := newTestPlatform(t, server.URL)
	buttons := [][]core.ButtonOption{
		{{Text: "Новый чат", Data: "cmd:/new"}, {Text: "Чаты", Data: "cmd:/list"}},
		{{Text: "gpt-6.1-sol", Data: "cmd:/model switch gpt-6.1-sol"}, {Text: "gpt-6.1-terra", Data: "cmd:/model switch gpt-6.1-terra"}},
		{{Text: "Низкая", Data: "cmd:/reasoning low"}, {Text: "Средняя", Data: "cmd:/reasoning medium"}, {Text: "Высокая", Data: "cmd:/reasoning high"}},
		{{Text: "Всё на эту сессию", Data: "perm:allow_all"}},
	}
	if err := p.SendWithButtons(context.Background(), replyContext{chatID: "555", userID: "99"}, "Меню", buttons); err != nil {
		t.Fatal(err)
	}
	want := []int{2, 1, 1, 2, 1, 1}
	if len(rows) != len(want) {
		t.Fatalf("unexpected rows: %#v", rows)
	}
	for index, row := range rows {
		if len(row) != want[index] {
			t.Errorf("row %d has %d buttons, want %d", index, len(row), want[index])
		}
		limit := 20 / len(row)
		for _, button := range row {
			if len([]rune(button.Text)) > limit {
				t.Errorf("fixture exceeds MAX recommendation: %q", button.Text)
			}
		}
	}
	cb := &maxCallback{Payload: rows[2][0].Payload, User: maxUser{UserID: 99}, Message: maxMessage{Body: maxBody{Mid: "compact-menu"}}}
	if _, action, ok := p.resolveButton(cb); !ok || action != "cmd:/model switch gpt-6.1-terra" {
		t.Fatalf("reflow changed the model action: %q %v", action, ok)
	}
}

func TestCallbackUpdateUsesTopLevelMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()
	p := newTestPlatform(t, server.URL)
	p.buttonMenus = map[string]buttonMenu{"nonce": {
		reply: replyContext{chatID: "555", userID: "99"}, messageID: "menu-1",
		expires: time.Now().Add(time.Minute), actions: []string{"cmd:/model"},
	}}
	var received *core.Message
	p.handler = func(_ core.Platform, msg *core.Message) { received = msg }
	var upd maxUpdate
	if err := json.Unmarshal([]byte(`{"update_type":"message_callback","callback":{"callback_id":"cb-1","payload":"ui:nonce:0","user":{"user_id":99}},"message":{"recipient":{"chat_id":555},"body":{"mid":"menu-1"}}}`), &upd); err != nil {
		t.Fatal(err)
	}
	p.handleUpdate(context.Background(), &upd)
	if received == nil || received.Content != "/model" || received.SessionKey != "max:555:99" {
		t.Fatalf("callback lost its original message: %#v", received)
	}
}

func TestUnboundDecisionCallbackIsRejected(t *testing.T) {
	p := newTestPlatform(t, "http://127.0.0.1:0")
	p.handler = func(_ core.Platform, _ *core.Message) { t.Error("unbound approval reached current request") }
	for _, payload := range []string{"perm:allow", "askq:0:1"} {
		p.handleCallback(context.Background(), &maxCallback{CallbackID: payload, Payload: payload, User: maxUser{UserID: 99}})
	}
}

func TestNewDecisionMenuInvalidatesOldPrompt(t *testing.T) {
	var payloads []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Attachments []struct {
				Payload struct{ Buttons [][]maxButton }
			}
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		payloads = append(payloads, body.Attachments[0].Payload.Buttons[0][0].Payload)
		_, _ = fmt.Fprintf(w, `{"message":{"body":{"mid":"prompt-%d"}}}`, len(payloads))
	}))
	defer server.Close()
	p := newTestPlatform(t, server.URL)
	ctx := context.Background()
	rctx := replyContext{chatID: "555", userID: "99"}
	for _, action := range []string{"perm:allow", "askq:0:1"} {
		if err := p.SendWithButtons(ctx, rctx, "Choose", [][]core.ButtonOption{{{Text: "OK", Data: action}}}); err != nil {
			t.Fatal(err)
		}
	}
	cb := &maxCallback{Payload: payloads[0], User: maxUser{UserID: 99}, Message: maxMessage{Body: maxBody{Mid: "prompt-1"}}}
	if _, _, ok := p.resolveButton(cb); ok {
		t.Fatal("old approval can affect a new pending request")
	}
	cb.Payload, cb.Message.Body.Mid = payloads[1], "prompt-2"
	if _, action, ok := p.resolveButton(cb); !ok || action != "askq:0:1" {
		t.Fatal("current decision rejected")
	}
}

func TestUploadKeepsTokenOnAPIOrigin(t *testing.T) {
	for _, redirect := range []bool{false, true} {
		t.Run(fmt.Sprint(redirect), func(t *testing.T) {
			cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" {
					t.Error("bot credential reached CDN")
				}
				_, _ = w.Write([]byte(`{"token":"uploaded"}`))
			}))
			defer cdn.Close()
			var api *httptest.Server
			api = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "test-token" {
					t.Error("API credential missing")
				}
				if r.URL.Path == "/redirect" {
					http.Redirect(w, r, cdn.URL, http.StatusTemporaryRedirect)
					return
				}
				uploadURL := cdn.URL
				if redirect {
					uploadURL = api.URL + "/redirect"
				}
				_ = json.NewEncoder(w).Encode(map[string]string{"url": uploadURL})
			}))
			defer api.Close()
			p := newTestPlatform(t, api.URL)
			if _, err := p.uploadAttachment(context.Background(), "file", []byte("test"), "test.txt"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBoundButtonsRequesterMessageAndReplay(t *testing.T) {
	var payload string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Attachments []struct {
				Payload struct{ Buttons [][]maxButton }
			}
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		payload = body.Attachments[0].Payload.Buttons[0][0].Payload
		_, _ = w.Write([]byte(`{"message":{"body":{"mid":"menu-1"}}}`))
	}))
	defer server.Close()
	p := newTestPlatform(t, server.URL)
	if err := p.SendWithButtons(context.Background(), replyContext{chatID: "555", userID: "99"},
		"Choose", [][]core.ButtonOption{{{Text: "Model", Data: "cmd:/model switch test-model"}, {Text: "Chats", Data: "cmd:/list"}}}); err != nil {
		t.Fatal(err)
	}
	cb := &maxCallback{Payload: payload, User: maxUser{UserID: 98}, Message: maxMessage{Body: maxBody{Mid: "menu-1"}}}
	if _, _, ok := p.resolveButton(cb); ok {
		t.Fatal("foreign user accepted")
	}
	cb.User.UserID = 99
	cb.Message.Body.Mid = "other"
	if _, _, ok := p.resolveButton(cb); ok {
		t.Fatal("foreign message accepted")
	}
	cb.Message.Body.Mid = "menu-1"
	cb.Message.Recipient.ChatID = 777
	if _, _, ok := p.resolveButton(cb); ok {
		t.Fatal("foreign chat accepted")
	}
	cb.Message.Recipient.ChatID = 0
	// Learning another chat must not redirect the original menu.
	p.chatOfUser.Store("99", "777")
	rctx, action, ok := p.resolveButton(cb)
	if !ok || rctx.chatID != "555" || action != "cmd:/model switch test-model" {
		t.Fatalf("wrong route/action: %#v %q %v", rctx, action, ok)
	}
	if _, _, ok := p.resolveButton(cb); ok {
		t.Fatal("replayed menu accepted")
	}
	cb.Payload = strings.TrimSuffix(payload, ":0") + ":1"
	if _, action, ok := p.resolveButton(cb); !ok || action != "cmd:/list" {
		t.Fatal("consuming one command disabled unrelated navigation")
	}
	if _, _, ok := newTestPlatform(t, server.URL).resolveButton(cb); ok {
		t.Fatal("menu survived loss of binding")
	}
}

func TestBoundButtonsExpired(t *testing.T) {
	p := newTestPlatform(t, "http://localhost")
	p.buttonMenus = map[string]buttonMenu{"expired": {reply: replyContext{userID: "99"}, messageID: "menu", actions: []string{"cmd:/new"}, expires: time.Now().Add(-time.Second)}}
	cb := &maxCallback{Payload: "ui:expired:0", User: maxUser{UserID: 99}, Message: maxMessage{Body: maxBody{Mid: "menu"}}}
	if _, _, ok := p.resolveButton(cb); ok {
		t.Fatal("expired menu accepted")
	}
}

func TestPreviewUsesBotMessageAndCleansUp(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		switch r.Method {
		case http.MethodPost:
			if r.URL.Query().Get("user_id") != "99" {
				t.Error("missing private-dialog fallback")
			}
			_, _ = w.Write([]byte(`{"message":{"body":{"mid":"preview-1"}}}`))
		case http.MethodPut, http.MethodDelete:
			if r.URL.Query().Get("message_id") != "preview-1" {
				t.Error("edited/deleted wrong message")
			}
			_, _ = w.Write([]byte(`{"success":true}`))
		}
	}))
	defer server.Close()
	p := newTestPlatform(t, server.URL)
	handle, err := p.SendPreviewStart(context.Background(), replyContext{userID: "99", messageID: "incoming"}, "First")
	if err != nil {
		t.Fatal(err)
	}
	if err = p.UpdateMessage(context.Background(), handle, "First second"); err != nil {
		t.Fatal(err)
	}
	if err = p.UpdateMessage(context.Background(), handle, strings.Repeat("ж", 4001)); err == nil {
		t.Fatal("oversize edit accepted")
	}
	if err = p.DeletePreviewMessage(context.Background(), handle); err != nil {
		t.Fatal(err)
	}
	if strings.Join(methods, ",") != "POST,PUT,DELETE" {
		t.Fatalf("unexpected requests: %v", methods)
	}
}

func TestCallbackCommandPrefixIsDispatched(t *testing.T) {
	p := newTestPlatform(t, "http://localhost")
	var content string
	p.handler = func(_ core.Platform, m *core.Message) { content = m.Content }
	p.handleCallback(context.Background(), &maxCallback{CallbackID: "test", Payload: "cmd:/model", User: maxUser{UserID: 99}, Message: maxMessage{Recipient: maxRecipient{ChatID: 555}}})
	if content != "/model" {
		t.Fatalf("got %q", content)
	}
}
