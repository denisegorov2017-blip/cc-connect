package core

import (
	"context"
	"strings"
	"testing"
	"time"
)

type captionTestSTT struct{}

func (captionTestSTT) Transcribe(context.Context, []byte, string, string) (string, error) {
	return "Распознанный текст", nil
}

func TestVoiceTranscriptionPreservesSeparateUserCaption(t *testing.T) {
	env := newCUJEnv(t)
	defer env.engine.Stop()
	env.engine.SetSpeechConfig(SpeechCfg{Enabled: true, STT: captionTestSTT{}})
	env.agent.setNextSessionEvents([]Event{{Type: EventResult, Content: "CAPTION-OK", Done: true}}, 0)
	env.engine.ReceiveMessage(env.plat, &Message{
		SessionKey: "test:caption", Platform: "test", MessageID: "voice-caption", UserID: "caption", ReplyCtx: "ctx",
		Audio: &AudioAttachment{Format: "wav", MimeType: "audio/wav", Data: []byte("fixture"), Caption: "Подпись пользователя"},
	})
	env.waitFor("caption transcript reply", 2*time.Second, func() bool { return env.sentContains("CAPTION-OK") })
	env.agent.mu.Lock()
	session := env.agent.sessions[0]
	env.agent.mu.Unlock()
	session.mu.Lock()
	defer session.mu.Unlock()
	if len(session.sentPrompts) != 1 || !strings.Contains(session.sentPrompts[0], "Подпись пользователя\nРаспознанный текст") {
		t.Fatalf("caption was lost or sent alone: %#v", session.sentPrompts)
	}
}
