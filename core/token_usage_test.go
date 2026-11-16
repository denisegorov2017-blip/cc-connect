package core

import (
	"strings"
	"testing"
	"time"
)

func TestProcessInteractiveEvents_ReportedContextWinsOverFixedWindowGuess(t *testing.T) {
	p := &stubPlatformEngine{n: "test"}
	agent := &stubReplyFooterAgent{stubModelModeAgent: stubModelModeAgent{model: "test-model"}}
	e := NewEngine("test", agent, []Platform{p}, "", LangEnglish)
	e.SetReplyFooterEnabled(true)
	key := "test:owner"
	session := e.sessions.GetOrCreateActive(key)
	as := newControllableSession("thread-1")
	as.contextUsage = &ContextUsage{UsedTokens: 34846, InputTokens: 34833, OutputTokens: 13, ContextWindow: 475000}
	state := &interactiveState{agentSession: as, platform: p, agent: agent}
	e.interactiveStates[key] = state
	as.events <- Event{Type: EventResult, Content: "answer", InputTokens: 34833, OutputTokens: 13, Done: true}
	e.processInteractiveEvents(state, session, e.sessions, key, "msg", time.Now(), nil, nil, nil)
	sent := strings.Join(p.getSent(), "\n")
	if !strings.Contains(sent, e.i18n.Tf(MsgReplyFooterRemaining, 93)) || strings.Contains(sent, "[ctx: ~17%]") {
		t.Fatalf("real context replaced with fixed-window guess: %s", sent)
	}
}
