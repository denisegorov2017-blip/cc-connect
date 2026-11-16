package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// A copy of this test executable acts as ffmpeg on every OS. No installed
// codec, shell, network listener, or real user recording is needed here.
func TestMain(m *testing.M) {
	if strings.EqualFold(strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe"), "ffmpeg") {
		switch os.Getenv("CC_CONNECT_TEST_FFMPEG_MODE") {
		case "hang":
			time.Sleep(20 * time.Second)
		case "large":
			_, _ = io.WriteString(os.Stdout, strings.Repeat("x", 25*1024*1024+1))
		case "private-error":
			_, _ = fmt.Fprint(os.Stderr, "PRIVATE-MEDIA-METADATA")
			os.Exit(7)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeSpeechFFmpeg(t *testing.T, mode string) {
	t.Helper()
	dir := t.TempDir()
	name := "ffmpeg"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(exe)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	target, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	_, copyErr := io.Copy(target, source)
	closeErr := target.Close()
	if copyErr != nil || closeErr != nil {
		t.Fatalf("copy=%v close=%v", copyErr, closeErr)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CC_CONNECT_TEST_FFMPEG_MODE", mode)
}

type speechBudgetStub struct{ called bool }

func (*speechBudgetStub) TranscriptionTimeout() time.Duration { return 150 * time.Millisecond }
func (s *speechBudgetStub) Transcribe(context.Context, []byte, string, string) (string, error) {
	s.called = true
	return "must not run after conversion deadline", nil
}

func TestTranscribeAudio_LocalBudgetIncludesConversion(t *testing.T) {
	fakeSpeechFFmpeg(t, "hang")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stt := &speechBudgetStub{}
	started := time.Now()
	_, err := TranscribeAudio(ctx, stt, &AudioAttachment{Data: []byte("unit-audio"), Format: "ogg"}, "ru")
	if time.Since(started) > time.Second || !errors.Is(err, context.DeadlineExceeded) || stt.called {
		t.Fatalf("elapsed=%s err=%v recognizer_called=%v", time.Since(started), err, stt.called)
	}
}

func TestConvertAudioToMP3_RejectsOversizedOutput(t *testing.T) {
	fakeSpeechFFmpeg(t, "large")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	data, err := ConvertAudioToMP3(ctx, []byte("unit-audio"), "ogg")
	if err == nil || len(data) != 0 || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("converted_bytes=%d err=%v", len(data), err)
	}
}

func TestConvertAudioToMP3_DoesNotReflectMediaMetadata(t *testing.T) {
	fakeSpeechFFmpeg(t, "private-error")
	_, err := ConvertAudioToMP3(context.Background(), []byte("unit-audio"), "ogg")
	if err == nil || strings.Contains(err.Error(), "PRIVATE-MEDIA-METADATA") {
		t.Fatalf("err=%v", err)
	}
}
