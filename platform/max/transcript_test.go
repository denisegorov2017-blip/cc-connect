package max

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chenhg5/cc-connect/core"
)

func TestNativeTranscriptSkipsAudioDownloadAndPreservesCaption(t *testing.T) {
	var downloads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/audio" {
			downloads.Add(1)
			w.WriteHeader(http.StatusGone)
			return
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()
	p := newTestPlatform(t, server.URL)
	var received []*core.Message
	p.handler = func(_ core.Platform, msg *core.Message) { received = append(received, msg) }
	var update maxUpdate
	fixture := fmt.Sprintf(`{"update_type":"message_created","message":{"timestamp":%d,"sender":{"user_id":99},"recipient":{"chat_id":555},"body":{"mid":"voice-native","text":"Подпись","attachments":[{"type":"audio","transcription":"  Тест голоса  ","payload":{"url":%q}},{"type":"audio","transcription":"Продолжение","payload":{}}]}}}`, time.Now().UnixMilli(), server.URL+"/audio")
	if err := json.Unmarshal([]byte(fixture), &update); err != nil {
		t.Fatal(err)
	}
	p.handleUpdate(context.Background(), &update)
	p.handleUpdate(context.Background(), &update)
	if len(received) != 1 {
		t.Fatalf("received %d messages, want one after duplicate", len(received))
	}
	if got := received[0]; got.Content != "Подпись\nТест голоса\nПродолжение" || got.Audio != nil || len(got.Files) != 0 {
		t.Fatalf("native transcript was not routed as text: %#v", got)
	}
	if downloads.Load() != 0 {
		t.Fatal("already transcribed audio was downloaded again")
	}
}

func TestNativeTranscriptFallbackRetainsMediaAndOriginalInput(t *testing.T) {
	for _, raw := range []string{`null`, `""`, `"  "`, `false`, `42`, `{}`, `[]`} {
		var attachment maxAttachmentRaw
		if err := json.Unmarshal([]byte(`{"type":"audio","transcription":`+raw+`,"payload":{"url":"https://media.invalid/audio"}}`), &attachment); err != nil {
			t.Fatal(err)
		}
		media, transcripts := separateNativeAudioTranscripts([]maxAttachmentRaw{attachment})
		if len(media) != 1 || len(transcripts) != 0 || media[0].Payload.URL != attachment.Payload.URL || string(media[0].Transcription) != raw {
			t.Errorf("fallback discarded media for %s", raw)
		}
	}
	attachments := []maxAttachmentRaw{
		{Type: "image", Transcription: json.RawMessage(`"Not a voice transcript"`), Payload: maxAttachmentPayld{URL: "https://media.invalid/image"}},
		{Type: "audio", Transcription: json.RawMessage(`"Тест голоса"`)},
		{Type: "file", Filename: "note.txt"},
		{Type: "audio", Payload: maxAttachmentPayld{URL: "https://media.invalid/untranscribed"}},
	}
	media, transcripts := separateNativeAudioTranscripts(attachments)
	if len(media) != 3 || len(transcripts) != 1 || media[0].Type != "image" || media[1].Filename != "note.txt" || media[2].Type != "audio" {
		t.Fatalf("mixed attachments changed: media=%#v transcripts=%#v", media, transcripts)
	}
	if attachments[1].Type != "audio" || string(attachments[1].Transcription) != `"Тест голоса"` {
		t.Fatal("normalization mutated the original update")
	}
}

func TestNativeTranscriptHonorsUserAllowList(t *testing.T) {
	p := newTestPlatform(t, "http://127.0.0.1:0")
	p.allowFrom = "123"
	p.handler = func(_ core.Platform, _ *core.Message) { t.Error("unauthorized transcript reached engine") }
	p.handleMessage(context.Background(), &maxMessage{
		Timestamp: time.Now().UnixMilli(), Sender: maxUser{UserID: 99},
		Body: maxBody{Mid: "unauthorized-voice", Attachments: []maxAttachmentRaw{{Type: "audio", Transcription: json.RawMessage(`"Тест голоса"`)}}},
	})
}
