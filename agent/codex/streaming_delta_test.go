package codex

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenhg5/cc-connect/core"
)

func TestAppServerStreamingDeltasAndCompletion(t *testing.T) {
	s := &appServerSession{ctx: context.Background(), streaming: true, events: make(chan core.Event, 16), currentTurn: "turn", streamedText: make(map[string]string)}
	s.threadID.Store("thread")
	for _, raw := range []string{
		`{"threadId":"other","turnId":"turn","itemId":"a","delta":"foreign"}`,
		`{"threadId":"thread","turnId":"old","itemId":"a","delta":"stale"}`,
		`{"threadId":"thread","turnId":"turn","itemId":"a","delta":"Hello"}`,
		`{"threadId":"thread","turnId":"turn","itemId":"a","delta":" world"}`,
	} {
		s.handleNotification("item/agentMessage/delta", json.RawMessage(raw))
	}
	if len(s.events) != 2 {
		t.Fatalf("events before completion=%d, want 2", len(s.events))
	}
	s.handleNotification("item/completed", json.RawMessage(`{"threadId":"thread","item":{"id":"a","type":"agentMessage","text":"Hello world!"}}`))
	s.handleNotification("item/completed", json.RawMessage(`{"threadId":"thread","turnId":"old","item":{"id":"old","type":"agentMessage","text":"stale completion"}}`))
	s.handleNotification("item/completed", json.RawMessage(`{"threadId":"thread","turnId":"turn","item":{"id":"a","type":"agentMessage","text":"Hello world!"}}`))
	s.handleNotification("thread/status/changed", json.RawMessage(`{"threadId":"thread","status":{"type":"idle"}}`))
	if s.currentTurn != "turn" {
		t.Fatal("idle event ended streaming before authoritative turn completion")
	}
	s.completeTurn()
	var text strings.Builder
	done := 0
	for len(s.events) > 0 {
		e := <-s.events
		switch e.Type {
		case core.EventText:
			text.WriteString(e.Content)
		case core.EventResult:
			done++
		}
	}
	if text.String() != "Hello world!" || done != 1 {
		t.Fatalf("text=%q done=%d", text.String(), done)
	}
}

func TestAppServerConnectUsesConfiguredExecutable(t *testing.T) {
	s := &appServerSession{ctx: context.Background(), cliBin: filepath.Join(t.TempDir(), "missing-codex-executable")}
	err := s.connect()
	if err == nil || !strings.Contains(err.Error(), "missing-codex-executable") {
		t.Fatalf("configured executable was ignored: %v", err)
	}
}

func TestAppServerStreamingFallbackWithoutDeltas(t *testing.T) {
	s := &appServerSession{ctx: context.Background(), streaming: true, events: make(chan core.Event, 4), currentTurn: "turn"}
	s.handleItemCompleted(map[string]any{"id": "a", "type": "agentMessage", "text": "Complete"})
	if len(s.events) != 1 {
		t.Fatal("complete message was not delivered")
	}
	s.completeTurn()
	first, second := <-s.events, <-s.events
	if first.Content != "Complete" || second.Type != core.EventResult {
		t.Fatalf("unexpected events: %#v %#v", first, second)
	}
}

func TestAppServerBufferedModeStillWaitsForCompletion(t *testing.T) {
	s := &appServerSession{ctx: context.Background(), events: make(chan core.Event, 4), currentTurn: "turn"}
	s.handleNotification("item/agentMessage/delta", json.RawMessage(`{"turnId":"turn","itemId":"a","delta":"ignored"}`))
	s.handleItemCompleted(map[string]any{"id": "a", "type": "agentMessage", "text": "Complete"})
	if len(s.events) != 0 {
		t.Fatal("buffered mode emitted early")
	}
	s.completeTurn()
	if (<-s.events).Content != "Complete" {
		t.Fatal("buffered final response lost")
	}
}
