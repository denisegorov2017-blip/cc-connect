package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceMenuBindsDirectoryWithSpaces(t *testing.T) {
	p := &stubInlineButtonPlatform{stubPlatformEngine: stubPlatformEngine{n: "inline"}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, filepath.Join(t.TempDir(), "sessions.json"), LangEnglish)
	root := t.TempDir()
	project := filepath.Join(root, "Project Alpha")
	if err := os.Mkdir(project, 0700); err != nil {
		t.Fatal(err)
	}
	e.SetMultiWorkspace(root, filepath.Join(t.TempDir(), "bindings.json"))
	msg := &Message{SessionKey: "inline:chat:user", UserID: "user", ReplyCtx: "ctx"}
	e.handleWorkspaceCommand(p, msg, []string{"list"})
	if len(p.buttonRows) != 1 || p.buttonRows[0][0].Data != "cmd:/workspace bind Project Alpha" {
		t.Fatalf("project choice missing: %#v", p.buttonRows)
	}
	e.ReceiveMessage(p, &Message{SessionKey: msg.SessionKey, UserID: msg.UserID, ReplyCtx: msg.ReplyCtx, Content: strings.TrimPrefix(p.buttonRows[0][0].Data, "cmd:")})
	binding := e.workspaceBindings.Lookup("project:test", effectiveWorkspaceChannelKey(msg))
	if binding == nil || binding.Workspace != normalizeWorkspacePath(project) {
		t.Fatalf("wrong project binding: %#v", binding)
	}
}

func TestSessionListButtonsUseStableIDs(t *testing.T) {
	p := &stubInlineButtonPlatform{stubPlatformEngine: stubPlatformEngine{n: "inline"}}
	agent := &stubListAgent{sessions: []AgentSessionInfo{{ID: "session-a", Summary: "Alpha"}, {ID: "session-b", Summary: "Beta"}}}
	e := NewEngine("test", agent, []Platform{p}, "", LangEnglish)
	msg := &Message{SessionKey: "inline:chat:user", ReplyCtx: "ctx"}
	e.cmdList(p, msg, nil)
	if len(p.buttonRows) < 2 || p.buttonRows[0][0].Data != "cmd:/switch session-a" {
		t.Fatalf("missing stable session button: %#v", p.buttonRows)
	}
	// Reordering the source list must leave the old button pointing to Alpha.
	agent.sessions[0], agent.sessions[1] = agent.sessions[1], agent.sessions[0]
	target := strings.TrimPrefix(p.buttonRows[0][0].Data, "cmd:/switch ")
	if got := e.matchSession(agent.sessions, e.sessions, target); got == nil || got.Summary != "Alpha" {
		t.Fatal("old button changed target")
	}
}

func TestSessionMenusAvailableWithEmptyListAndWelcome(t *testing.T) {
	p := &stubInlineButtonPlatform{stubPlatformEngine: stubPlatformEngine{n: "inline"}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	msg := &Message{SessionKey: "inline:chat:user", ReplyCtx: "ctx"}
	e.cmdList(p, msg, nil)
	if len(p.buttonRows) != 3 || p.buttonRows[0][0].Data != "cmd:/new" {
		t.Fatal("empty list lacks new-chat action")
	}
	e.ReceiveMessage(p, &Message{SessionKey: msg.SessionKey, ReplyCtx: msg.ReplyCtx, Content: "/start"})
	if p.buttonRows[1][0].Data != "cmd:/model" || p.buttonRows[1][1].Data != "cmd:/reasoning" {
		t.Fatal("welcome lacks model/reasoning menu")
	}
}
