package maxcodex_test

// Real Codex inference, local MAX transport fixture. Never polls the user's bot.
import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/chenhg5/cc-connect/agent/codex"
	"github.com/chenhg5/cc-connect/core"
	maxplatform "github.com/chenhg5/cc-connect/platform/max"
)

const benchmarkWaitPolicy = "Для уточняющих вопросов с вариантами ответа используйте request_user_input, когда он доступен; иначе request_user_input_async."
const benchmarkQuestion = "Это изолированная проверка кнопок MAX. Никаких файлов, платежей и других инструментов. Используй доступный инструмент структурированных вопросов. Один вопрос: «Тест MAX: какой вид операции?» и два варианта «Расход» и «Приход». Не выбирай ответ сам. После получения ответа «Расход» ответь ровно MAX-BUTTONS-OK:Расход. Если инструмент недоступен, сообщи об этом явно. Не печатай маркер до ответа пользователя."

type benchmarkAgent struct {
	core.Agent
	mu      sync.Mutex
	session core.AgentSession
}

func (a *benchmarkAgent) StartSession(ctx context.Context, id string) (core.AgentSession, error) {
	s, err := a.Agent.StartSession(ctx, id)
	if err == nil {
		a.mu.Lock()
		a.session = s
		a.mu.Unlock()
	}
	return s, err
}

func TestContextQuestionBenchmark(t *testing.T) {
	if os.Getenv("CC_CONTEXT_BENCHMARK") != "1" {
		t.Skip("explicit real-inference benchmark")
	}
	root, variant := os.Getenv("CC_CONTEXT_BENCHMARK_ROOT"), os.Getenv("CC_CONTEXT_VARIANT")
	if root == "" || (variant != "before" && variant != "after") {
		t.Fatal("missing isolated benchmark configuration")
	}
	var cfg struct {
		Projects []struct {
			Agent struct{ Options map[string]any }
		}
	}
	if _, err := toml.DecodeFile(os.Getenv("CC_CONTEXT_BASE_CONFIG"), &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Projects) != 1 {
		t.Fatal("ambiguous baseline config")
	}
	base, _ := cfg.Projects[0].Agent.Options["append_system_prompt"].(string)
	appendPrompt := base
	if variant == "after" {
		appendPrompt = strings.Replace(base, "Для уточняющих вопросов с вариантами ответа используйте доступный request_user_input_async; если его нет, используйте request_user_input, когда он доступен.", benchmarkWaitPolicy, 1)
		if appendPrompt == base {
			t.Fatal("baseline question policy changed; inspect it before benchmarking")
		}
	}
	cmd := []string{os.Getenv("CC_CODEX_BINARY"), "-c", "features.default_mode_request_user_input=true"}
	ag, err := codex.New(map[string]any{"cmd": cmd, "work_dir": root, "backend": "app_server", "app_server_url": "stdio://", "model": "gpt-6.1-sol", "reasoning_effort": "low", "mode": "suggest", "append_system_prompt": appendPrompt})
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join("..", "..", "agent", "codex", "appserver_session.go"))
	if err != nil {
		t.Fatal(err)
	}
	a := &benchmarkAgent{Agent: ag}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	updates := make(chan map[string]any, 4)
	done := make(chan struct{}, 1)
	var mu sync.Mutex
	var rows [][]struct{ Type, Text, Payload string }
	var mid string
	var sent, callbacks int
	var questionAt, answeredAt time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/me":
			fmt.Fprint(w, `{"user_id":1,"name":"isolated MAX fixture"}`)
		case "/updates":
			select {
			case u := <-updates:
				_ = json.NewEncoder(w).Encode(map[string]any{"updates": []any{u}})
			case <-r.Context().Done():
			case <-ctx.Done():
				fmt.Fprint(w, `{"updates":[]}`)
			case <-time.After(time.Second):
				fmt.Fprint(w, `{"updates":[]}`)
			}
		case "/messages":
			if r.Method == http.MethodPut {
				fmt.Fprint(w, `{"success":true}`)
				return
			}
			var body struct {
				Text        string
				Attachments []struct {
					Type    string
					Payload struct {
						Buttons [][]struct{ Type, Text, Payload string }
					}
				}
			}
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				http.Error(w, "bad request", 400)
				return
			}
			mu.Lock()
			sent++
			messageID := fmt.Sprintf("fixture-%d", sent)
			if len(body.Attachments) > 0 && body.Attachments[0].Type == "inline_keyboard" {
				rows = body.Attachments[0].Payload.Buttons
				mid = messageID
				questionAt = time.Now()
				var choice string
				for _, row := range rows {
					for _, b := range row {
						if strings.HasPrefix(b.Text, "Расход") {
							choice = b.Payload
						}
					}
				}
				if choice == "" {
					t.Error("expected Russian choice missing")
				}
				go func(payload, messageID string) {
					select {
					case <-ctx.Done():
						return
					case <-time.After(12 * time.Second):
					}
					mu.Lock()
					callbacks++
					answeredAt = time.Now()
					mu.Unlock()
					updates <- map[string]any{"update_type": "message_callback", "callback": map[string]any{"callback_id": "fixture-choice", "payload": payload, "user": map[string]any{"user_id": 99, "name": "test"}}, "message": map[string]any{"recipient": map[string]any{"chat_id": 555}, "body": map[string]any{"mid": messageID, "text": body.Text}}}
				}(choice, messageID)
			}
			accepted := callbacks == 1
			mu.Unlock()
			if strings.Contains(body.Text, "MAX-BUTTONS-OK:Расход") && !accepted {
				t.Error("model answered before the user selected a button")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"message": map[string]any{"body": map[string]any{"mid": messageID}}})
			if accepted && strings.Contains(body.Text, "MAX-BUTTONS-OK:Расход") {
				select {
				case done <- struct{}{}:
				default:
				}
			}
		default:
			fmt.Fprint(w, `{"success":true}`)
		}
	}))
	defer server.Close()
	p, err := maxplatform.New(map[string]any{"token": "fixture-not-a-bot-token", "allow_from": "99", "api_base": server.URL})
	if err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(root, "benchmark-"+variant+"-sessions.json")
	if _, err := os.Stat(store); err == nil {
		t.Fatal("benchmark output exists; do not overwrite uncertain run")
	}
	e := core.NewEngine("context-benchmark", a, []core.Platform{p}, store, core.LangRussian)
	e.SetStreamPreviewCfg(core.StreamPreviewCfg{Enabled: false})
	if err := e.Start(); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	r, _ := p.(core.ReplyContextReconstructor).ReconstructReplyCtx("max:555:99")
	start := time.Now()
	e.ReceiveMessage(p, &core.Message{SessionKey: "max:555:99", Platform: "max", UserID: "99", ReplyCtx: r, Content: benchmarkQuestion})
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("same-session final result did not arrive")
	}
	a.mu.Lock()
	as := a.session
	a.mu.Unlock()
	reporter, ok := as.(interface {
		GetTokenUsageSnapshot() core.TokenUsageSnapshot
	})
	if !ok {
		t.Fatal("missing native usage")
	}
	snapshot := reporter.GetTokenUsageSnapshot()
	if snapshot.Session == nil {
		t.Fatal("no measured counters")
	}
	mu.Lock()
	report := map[string]any{"variant": variant, "source_file_sha256": fmt.Sprintf("%x", sha256.Sum256(source)), "effective_append_prompt_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(appendPrompt))), "source_commit": os.Getenv("CC_CONTEXT_SOURCE"), "model": "gpt-6.1-sol", "effort": "low", "prompt_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(benchmarkQuestion))), "base_prompt_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(base))), "policy_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(benchmarkWaitPolicy))), "thread_id": snapshot.ThreadID, "session_tokens": snapshot.Session, "last_request_tokens": snapshot.LastRequest, "button_rows": rows, "question_mid": mid, "callbacks": callbacks, "wall_ms": time.Since(start).Milliseconds(), "question_ms": questionAt.Sub(start).Milliseconds(), "controlled_answer_delay_ms": answeredAt.Sub(questionAt).Milliseconds(), "final_after_answer_ms": time.Since(answeredAt).Milliseconds(), "real_max_network": false, "real_codex_inference": true, "result": "MAX-BUTTONS-OK:Расход"}
	mu.Unlock()
	data, _ := json.MarshalIndent(report, "", "  ")
	if err := os.WriteFile(filepath.Join(root, "result-"+variant+".json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: input=%d cached=%d output=%d total=%d thread=%s", variant, snapshot.Session.InputTokens, snapshot.Session.CachedInputTokens, snapshot.Session.OutputTokens, snapshot.Session.TotalTokens, snapshot.ThreadID)
}
