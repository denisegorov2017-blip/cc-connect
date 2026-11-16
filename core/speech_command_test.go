package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func speechTestCommand(t *testing.T, mode string) []string {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return []string{path, "-test.run=^TestCommandSTTChildProcess$", "--", "speech-command-helper", mode}
}

func TestCommandSTTChildProcess(t *testing.T) {
	var mode string
	for i, arg := range os.Args {
		if arg == "speech-command-helper" && i+1 < len(os.Args) {
			mode = os.Args[i+1]
		}
	}
	if mode == "" {
		return
	}
	switch mode {
	case "success":
		data, _ := io.ReadAll(os.Stdin)
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"text": string(data) + "|" + os.Getenv("CC_CONNECT_STT_FORMAT") + "|" + os.Getenv("CC_CONNECT_STT_LANGUAGE")})
	case "fail":
		_, _ = fmt.Fprintln(os.Stderr, "PRIVATE-STDERR-MUST-NOT-LEAK")
		os.Exit(7)
	case "invalid":
		_, _ = fmt.Fprint(os.Stdout, `{"transcript":"wrong contract"}`)
	case "oversize":
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"text": strings.Repeat("x", maxCommandSpeechOutput+1)})
	case "hang":
		time.Sleep(30 * time.Second)
	}
	os.Exit(0)
}

func TestCommandSTT_StdinAndMetadata(t *testing.T) {
	t.Setenv("CC_CONNECT_STT_LANGUAGE", "stale")
	text, err := NewCommandSTT(speechTestCommand(t, "success"), time.Second*5).Transcribe(
		context.Background(), []byte("Тест голоса; $(literal data)"), "mp3", "ru")
	if err != nil || text != "Тест голоса; $(literal data)|mp3|ru" {
		t.Fatalf("text=%q err=%v", text, err)
	}
}

func TestCommandSTT_FailuresAreBoundedAndDoNotLeakStderr(t *testing.T) {
	for _, mode := range []string{"fail", "invalid", "oversize"} {
		t.Run(mode, func(t *testing.T) {
			text, err := NewCommandSTT(speechTestCommand(t, mode), 5*time.Second).Transcribe(context.Background(), []byte("audio"), "mp3", "ru")
			if err == nil || text != "" || strings.Contains(err.Error(), "PRIVATE") {
				t.Fatalf("text_bytes=%d err=%v", len(text), err)
			}
		})
	}
}

func TestCommandSTT_TimeoutAndCancellation(t *testing.T) {
	_, err := NewCommandSTT(speechTestCommand(t, "hang"), 150*time.Millisecond).Transcribe(context.Background(), []byte("audio"), "mp3", "ru")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout err=%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = NewCommandSTT(speechTestCommand(t, "success"), time.Second).Transcribe(ctx, []byte("audio"), "mp3", "ru")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel err=%v", err)
	}
}

func TestCommandSTT_InvalidInputDoesNotStartProcess(t *testing.T) {
	for _, data := range [][]byte{nil, make([]byte, 25*1024*1024+1)} {
		_, err := NewCommandSTT([]string{"executable-must-not-run"}, time.Second).Transcribe(context.Background(), data, "mp3", "ru")
		if err == nil || !strings.Contains(err.Error(), "size") {
			t.Fatalf("err=%v", err)
		}
	}
}

func TestCUJ_A4_CommandSpeechFailureThenRecovery(t *testing.T) {
	env := newCUJEnv(t)
	defer env.engine.Stop()
	send := func(id string) {
		env.engine.ReceiveMessage(env.plat, &Message{SessionKey: "test:command-voice", Platform: "test", MessageID: id,
			UserID: "voice", UserName: "voice", ReplyCtx: "ctx",
			Audio: &AudioAttachment{Format: "mp3", MimeType: "audio/mpeg", Data: []byte("VOICE-INPUT"), Caption: "CAPTION"}})
	}
	// 1. No recognizer: a visible explanation and no agent call.
	send("unconfigured")
	if env.lastSent() != env.engine.i18n.T(MsgVoiceNotEnabled) {
		t.Fatalf("no explanation: %q", env.lastSent())
	}
	// 2. A failed local process must not route a caption-only request.
	env.engine.SetSpeechConfig(SpeechCfg{Enabled: true, Language: "ru", STT: NewCommandSTT(speechTestCommand(t, "fail"), 5*time.Second)})
	send("failed")
	env.waitFor("recognition failure", 5*time.Second, func() bool { return env.sentContains("speech command failed") })
	env.agent.mu.Lock()
	sessions := len(env.agent.sessions)
	env.agent.mu.Unlock()
	if sessions != 0 {
		t.Fatal("failed audio reached agent")
	}
	// 3. Reconfigured recognition sends both caption and speech to the same chat.
	env.agent.setNextSessionEvents([]Event{{Type: EventResult, Content: "LOCAL-VOICE-OK", Done: true}}, 0)
	env.engine.SetSpeechConfig(SpeechCfg{Enabled: true, Language: "ru", STT: NewCommandSTT(speechTestCommand(t, "success"), 5*time.Second)})
	send("recovered")
	env.waitFor("voice agent reply", 5*time.Second, func() bool { return env.sentContains("LOCAL-VOICE-OK") })
	env.agent.mu.Lock()
	prompts := env.agent.sessions[0].getSentPrompts()
	env.agent.mu.Unlock()
	if len(prompts) != 1 || !strings.Contains(prompts[0], "CAPTION") || !strings.Contains(prompts[0], "VOICE-INPUT|mp3|ru") {
		t.Fatalf("voice/caption lost: %v", prompts)
	}
}
