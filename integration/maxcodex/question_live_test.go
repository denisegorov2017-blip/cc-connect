package maxcodex_test

// Opt-in live boundary check. Use an isolated work directory and stop any other
// poller for this bot first. No credentials are persisted by this test.
import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chenhg5/cc-connect/agent/codex"
	"github.com/chenhg5/cc-connect/core"
	maxplatform "github.com/chenhg5/cc-connect/platform/max"
)

type observedPlatform struct {
	core.Platform
	t        *testing.T
	done     chan struct{}
	once     sync.Once
	answered bool
	mu       sync.Mutex
}

func (p *observedPlatform) observe(text string) {
	p.mu.Lock()
	answered := p.answered
	p.mu.Unlock()
	if answered && strings.Contains(text, "MAX-BUTTONS-OK:Расход") {
		p.once.Do(func() { close(p.done) })
	}
}

func (p *observedPlatform) Start(h core.MessageHandler) error {
	return p.Platform.Start(func(_ core.Platform, msg *core.Message) {
		if msg.IsPermissionResponse {
			p.mu.Lock()
			p.answered = true
			p.mu.Unlock()
			p.t.Logf("MAX callback: session=%s content=%s", msg.SessionKey, msg.Content)
		}
		h(p, msg)
	})
}

func (p *observedPlatform) Send(ctx context.Context, r any, text string) error {
	err := p.Platform.Send(ctx, r, text)
	if err == nil {
		p.observe(text)
	}
	return err
}

func (p *observedPlatform) Reply(ctx context.Context, r any, text string) error {
	err := p.Platform.Reply(ctx, r, text)
	if err == nil {
		p.observe(text)
	}
	return err
}

func (p *observedPlatform) SendWithButtons(ctx context.Context, r any, text string, buttons [][]core.ButtonOption) error {
	err := p.Platform.(core.InlineButtonSender).SendWithButtons(ctx, r, text, buttons)
	if err == nil {
		p.t.Logf("MAX inline keyboard delivered: %s; rows=%d", text, len(buttons))
	}
	return err
}

func TestLiveCodexQuestionMAXRoundTrip(t *testing.T) {
	if os.Getenv("CC_MAX_QUESTION_LIVE") != "1" {
		t.Skip("explicit live test only")
	}
	root, token, owner, key := os.Getenv("CC_MAX_PROBE_ROOT"), os.Getenv("MAX_BOT_TOKEN"), os.Getenv("CC_MAX_OWNER"), os.Getenv("CC_MAX_SESSION_KEY")
	if root == "" || token == "" || owner == "" || key == "" {
		t.Fatal("missing isolated live test configuration")
	}
	ag, err := codex.New(map[string]any{
		"cmd": []string{os.Getenv("CC_CODEX_BINARY"), "-c", "features.default_mode_request_user_input=true"}, "work_dir": root, "backend": "app_server", "app_server_url": "stdio://",
		"model": "gpt-6.1-sol", "reasoning_effort": "low", "mode": "suggest",
	})
	if err != nil {
		t.Fatal(err)
	}
	inner, err := maxplatform.New(map[string]any{"token": token, "allow_from": owner, "api_base": "https://platform-api2.max.ru"})
	if err != nil {
		t.Fatal(err)
	}
	p := &observedPlatform{Platform: inner, t: t, done: make(chan struct{})}
	e := core.NewEngine("isolated-question-probe", ag, []core.Platform{p}, filepath.Join(root, "sessions.json"), core.LangRussian)
	if err := e.Start(); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	r, err := inner.(core.ReplyContextReconstructor).ReconstructReplyCtx(key)
	if err != nil {
		t.Fatal(err)
	}
	e.ReceiveMessage(p, &core.Message{SessionKey: key, Platform: "max", UserID: owner, ReplyCtx: r,
		Content: "Это изолированная проверка кнопок MAX. Никаких файлов, платежей и других инструментов. Используй доступный инструмент структурированных вопросов: request_user_input_async, а если его нет — request_user_input. Один вопрос: «Тест MAX: какой вид операции?» и два варианта «Расход» и «Приход». Не выбирай ответ сам. После получения ответа «Расход» ответь ровно MAX-BUTTONS-OK:Расход. Если инструмент недоступен, сообщи об этом явно. Не печатай маркер до ответа пользователя.",
	})
	select {
	case <-p.done:
		t.Log("final confirmation delivered; inspect sessions.json and daemon trace for original thread identity")
	case <-time.After(3 * time.Minute):
		t.Fatal("no confirmed round trip before deadline")
	}
}
