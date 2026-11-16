package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenhg5/cc-connect/core"
)

func TestCompleteTurn_ReportsActualUsageInsteadOfZero(t *testing.T) {
	s := &appServerSession{events: make(chan core.Event, 8)}
	s.threadID.Store("thread-1")
	s.handleNotification("turn/started", json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1"}}`))
	s.handleNotification("thread/tokenUsage/updated", json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","tokenUsage":{"total":{"inputTokens":34716,"cachedInputTokens":0,"outputTokens":40,"reasoningOutputTokens":0,"totalTokens":34756},"last":{"inputTokens":34716,"cachedInputTokens":0,"outputTokens":40,"reasoningOutputTokens":0,"totalTokens":34756},"modelContextWindow":475000}}`))
	s.completeTurn()
	for len(s.events) > 0 {
		event := <-s.events
		if event.Type == core.EventResult {
			if event.InputTokens != 34716 || event.OutputTokens != 40 {
				t.Fatalf("actual usage lost: input=%d output=%d", event.InputTokens, event.OutputTokens)
			}
			return
		}
	}
	t.Fatal("missing result")
}

func TestTokenMeter_ReplaysRecordedQuestionWithoutDoubleCounting(t *testing.T) {
	m := appServerTokenMeter{}
	m.begin("turn-1")
	updates := [][6]int{{34716, 0, 40, 34716, 0, 40}, {69490, 34560, 59, 34774, 34560, 19}, {104323, 69120, 72, 34833, 34560, 13}}
	for _, u := range updates {
		n := usageNotification("turn-1", u[0], u[1], u[2], u[3], u[4], u[5])
		m.observe(n)
		m.observe(n) // delivery retry / rate-limit event with unchanged counters
	}
	if m.turn == nil || m.turn.InputTokens != 104323 || m.turn.CachedInputTokens != 69120 || m.turn.OutputTokens != 72 || m.turn.TotalTokens != 104395 {
		t.Fatalf("recorded turn counters wrong: %#v", m.turn)
	}
	if m.last.TotalTokens != 34846 {
		t.Fatal("last request confused with total turn")
	}
	m.begin("turn-2")
	m.observe(usageNotification("turn-1", 999999, 0, 99, 999999, 0, 99))
	m.observe(usageNotification("turn-2", 104523, 69270, 82, 200, 150, 10))
	if m.turn.TotalTokens != 210 || m.turn.CachedInputTokens != 150 {
		t.Fatalf("new turn included old usage: %#v", m.turn)
	}
}

func usageNotification(turn string, input, cached, output, lastInput, lastCached, lastOutput int) appServerThreadTokenUsageNotification {
	n := appServerThreadTokenUsageNotification{ThreadID: "thread-1", TurnID: turn}
	n.TokenUsage.Total = codexTokenUsage{InputTokens: input, CachedInputTokens: cached, OutputTokens: output, TotalTokens: input + output}
	n.TokenUsage.Last = codexTokenUsage{InputTokens: lastInput, CachedInputTokens: lastCached, OutputTokens: lastOutput, TotalTokens: lastInput + lastOutput}
	return n
}

func TestTokenMeter_UnknownResumeBaselineIsNotInvented(t *testing.T) {
	m := appServerTokenMeter{}
	m.begin("resumed")
	m.observe(usageNotification("resumed", 10000, 5000, 100, 100, 50, 10))
	if m.turn != nil || m.total.InputTokens != 10000 {
		t.Fatal("unknown turn baseline was estimated")
	}
}

func TestTokenMeter_RejectsForeignThreadAndMalformedCounters(t *testing.T) {
	s := &appServerSession{}
	s.threadID.Store("thread-1")
	s.tokenMeter.begin("turn-1")
	n := usageNotification("turn-1", 100, 50, 10, 100, 50, 10)
	n.ThreadID = "foreign"
	raw, _ := json.Marshal(n)
	s.handleNotification("thread/tokenUsage/updated", raw)
	if s.GetTokenUsageSnapshot().Session != nil {
		t.Fatal("foreign counters accepted")
	}
	n.ThreadID = "thread-1"
	n.TokenUsage.Total.CachedInputTokens = 101
	s.tokenMeter.observe(n)
	if s.GetTokenUsageSnapshot().Session != nil {
		t.Fatal("malformed counters accepted")
	}
}

func TestReadSessionTokenUsage_RestoresTurnFromJournalWithoutModel(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "sessions", "2026", "11", "16")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-test-thread-1.jsonl")
	data := `{"type":"turn_context","payload":{"turn_id":"turn-1"}}
{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":50,"cache_write_input_tokens":4,"output_tokens":10,"reasoning_output_tokens":2,"total_tokens":110},"last_token_usage":{"input_tokens":100,"cached_input_tokens":50,"cache_write_input_tokens":4,"output_tokens":10,"reasoning_output_tokens":2,"total_tokens":110},"model_context_window":1000}}}
{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":300,"cached_input_tokens":150,"cache_write_input_tokens":6,"output_tokens":30,"reasoning_output_tokens":5,"total_tokens":330},"last_token_usage":{"input_tokens":200,"cached_input_tokens":100,"cache_write_input_tokens":2,"output_tokens":20,"reasoning_output_tokens":3,"total_tokens":220},"model_context_window":1000}}}
`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	a := &Agent{codexHome: home}
	snapshot, err := readSessionTokensForTest(a, "thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.TurnID != "turn-1" || snapshot.Turn.TotalTokens != 330 || snapshot.Session.CacheWriteInputTokens != 6 || snapshot.LastRequest.TotalTokens != 220 {
		t.Fatalf("bad restored counters: %#v", snapshot)
	}
}

func TestTokenMeter_RecordedMAXJournal(t *testing.T) {
	if os.Getenv("CC_TOKEN_METER_REPLAY") != "1" {
		t.Skip("explicit retained-journal verification")
	}
	a := &Agent{codexHome: os.Getenv("CC_TOKEN_METER_CODEX_HOME")}
	snapshot, err := readSessionTokensForTest(a, "01a1e479-8751-7830-af2e-5b8a03f2b17a")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Turn == nil || snapshot.Turn.InputTokens != 104323 || snapshot.Turn.CachedInputTokens != 69120 || snapshot.Turn.OutputTokens != 72 || snapshot.Turn.TotalTokens != 104395 {
		t.Fatalf("retained native journal mismatch: %#v", snapshot.Turn)
	}
	t.Logf("native journal: input=%d cached=%d uncached=%d output=%d total=%d; last request=%d", snapshot.Turn.InputTokens, snapshot.Turn.CachedInputTokens, snapshot.Turn.InputTokens-snapshot.Turn.CachedInputTokens, snapshot.Turn.OutputTokens, snapshot.Turn.TotalTokens, snapshot.LastRequest.TotalTokens)
}

func readSessionTokensForTest(a *Agent, sessionID string) (core.TokenUsageSnapshot, error) {
	env := append([]string(nil), a.configEnv...)
	if a.codexHome != "" {
		env = append(env, "CODEX_HOME="+a.codexHome)
	}
	usage, path, err := loadContextUsageFromRollout(env, sessionID, "")
	if err != nil {
		return core.TokenUsageSnapshot{}, err
	}
	if usage == nil || usage.Tokens == nil {
		return core.TokenUsageSnapshot{}, fmt.Errorf("codex: token counters unavailable")
	}
	snapshot := usage.Tokens.Clone()
	snapshot.ThreadID = sessionID
	if f, openErr := os.Open(path); openErr == nil {
		defer f.Close()
		// Cumulative counters came from the tail; bound optional turn reconstruction.
		if info, statErr := f.Stat(); statErr == nil && info.Size() <= 64<<20 {
			if recovered, scanErr := scanTokenMeter(f); scanErr == nil {
				snapshot.TurnID, snapshot.Turn = recovered.turnID, copyTokenCounts(recovered.turn)
				if recovered.total != nil {
					snapshot.Session, snapshot.LastRequest = copyTokenCounts(recovered.total), copyTokenCounts(recovered.last)
				}
			}
		}
	}
	return snapshot, nil
}

func scanTokenMeter(f *os.File) (appServerTokenMeter, error) {
	var meter appServerTokenMeter
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		var entry struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if json.Unmarshal(scanner.Bytes(), &entry) != nil {
			continue
		}
		if entry.Type != "turn_context" && entry.Type != "event_msg" {
			continue
		}
		var payload struct {
			Type   string `json:"type"`
			TurnID string `json:"turn_id"`
			Info   *struct {
				Total codexSnakeTokenUsage `json:"total_token_usage"`
				Last  codexSnakeTokenUsage `json:"last_token_usage"`
			} `json:"info"`
		}
		if json.Unmarshal(entry.Payload, &payload) != nil {
			continue
		}
		if entry.Type == "turn_context" || payload.Type == "task_started" {
			meter.begin(payload.TurnID)
			continue
		}
		if payload.Type != "token_count" || payload.Info == nil {
			continue
		}
		convert := func(u codexSnakeTokenUsage) codexTokenUsage {
			return codexTokenUsage{InputTokens: u.InputTokens, CachedInputTokens: u.CachedInputTokens, CacheWriteInputTokens: u.CacheWriteInputTokens, OutputTokens: u.OutputTokens, ReasoningOutputTokens: u.ReasoningOutputTokens, TotalTokens: u.TotalTokens}
		}
		n := appServerThreadTokenUsageNotification{TurnID: meter.turnID}
		n.TokenUsage.Total, n.TokenUsage.Last = convert(payload.Info.Total), convert(payload.Info.Last)
		meter.observe(n)
	}
	return meter, scanner.Err()
}
