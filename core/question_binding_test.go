package core

import (
	"sync"
	"testing"
)

func TestHandlePendingPermission_StaleQuestionIndexCannotAnswerNextQuestion(t *testing.T) {
	e := newTestEngine()
	p := &stubPlatformEngine{n: "test"}
	rec := &recordingAgentSession{}
	pending := &pendingPermission{RequestID: "request", Questions: testMultiQuestions(), CurrentQuestion: 1, Answers: map[int]string{0: "PostgreSQL"}, Resolved: make(chan struct{})}
	e.interactiveStates["test:chat:user1"] = &interactiveState{agentSession: rec, platform: p, pending: pending}
	msg := &Message{SessionKey: "test:chat:user1", UserID: "user1", Content: "askq:0:2", IsPermissionResponse: true}
	e.ReceiveMessage(p, msg)
	if rec.calls != 0 {
		t.Fatal("old question button answered the next question")
	}
}

func TestQuestionBinding_RejectsOtherRequestSessionAndDuplicate(t *testing.T) {
	e := newTestEngine()
	p := &stubPlatformEngine{n: "test"}
	rec := &recordingAgentSession{}
	pending := &pendingPermission{RequestID: "original-rpc-id", QuestionToken: "original-token", Questions: testQuestions(), Resolved: make(chan struct{})}
	e.interactiveStates["test:chat:user1"] = &interactiveState{agentSession: rec, platform: p, pending: pending}
	for _, content := range []string{"askq:old-token:0:1", "askq:original-token:1:1", "askq:original-token:0:99", "askq:0:1"} {
		e.ReceiveMessage(p, &Message{SessionKey: "test:chat:user1", Content: content, IsPermissionResponse: true})
	}
	e.ReceiveMessage(p, &Message{SessionKey: "test:other:user2", Content: "askq:original-token:0:2", IsPermissionResponse: true})
	if rec.calls != 0 {
		t.Fatal("unrelated callback answered original request")
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			e.ReceiveMessage(p, &Message{SessionKey: "test:chat:user1", Content: "askq:original-token:0:2", IsPermissionResponse: true})
		})
	}
	wg.Wait()
	if rec.calls != 1 || rec.lastID != "original-rpc-id" {
		t.Fatalf("response was not delivered exactly once to original request: calls=%d id=%s", rec.calls, rec.lastID)
	}
	if rec.lastResult.UpdatedInput["answers"].(map[string]any)["Which database?"] != "SQLite" {
		t.Fatal("selected option label was not returned")
	}
}
