package codex

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chenhg5/cc-connect/core"
)

func TestLiveAppServerStreamingRoundTrip(t *testing.T) {
	bin := os.Getenv("CC_CONNECT_LIVE_CODEX_BIN")
	if bin == "" {
		t.Skip("set CC_CONNECT_LIVE_CODEX_BIN to test an authenticated native Codex")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	agent, err := New(map[string]any{
		"cmd": []string{bin}, "work_dir": t.TempDir(), "mode": "suggest",
		"backend": "app_server", "app_server_url": "stdio", "streaming": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := agent.StartSession(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	started := time.Now()
	if err := session.Send("Reply with exactly CC_CONNECT_NATIVE_OK. Do not call tools.", "native-smoke", nil, nil); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	for {
		select {
		case event, ok := <-session.Events():
			if !ok {
				t.Fatal("event stream closed before turn completion")
			}
			switch event.Type {
			case core.EventText:
				if output.Len() == 0 {
					t.Logf("first text after %s", time.Since(started))
				}
				output.WriteString(event.Content)
			case core.EventError:
				t.Fatalf("native turn failed: %v", event.Error)
			case core.EventResult:
				if !event.Done {
					continue
				}
				if strings.TrimSpace(output.String()) != "CC_CONNECT_NATIVE_OK" {
					t.Fatalf("unexpected or duplicated native output %q", output.String())
				}
				t.Logf("native turn completed after %s", time.Since(started))
				return
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
}
