package config

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestSpeechCommandDecodeAndValidation(t *testing.T) {
	var cfg Config
	_, err := toml.Decode(`[speech]
enabled = true
provider = "command"
language = "ru"
[speech.command]
cmd = ['C:\Program Files\Python\python.exe', 'local-recognizer.py']
timeout_seconds = 45
`, &cfg)
	if err != nil || len(cfg.Speech.Command.Cmd) != 2 || cfg.Speech.Command.Cmd[0] != `C:\Program Files\Python\python.exe` || cfg.Speech.Command.TimeoutSeconds != 45 {
		t.Fatalf("cfg=%+v err=%v", cfg.Speech, err)
	}
	cfg.Projects = []ProjectConfig{validProject("voice")}
	if err := cfg.validate(); err != nil {
		t.Fatal(err)
	}
	cfg.Speech.Command.TimeoutSeconds = 121
	if err := cfg.validate(); err == nil || !strings.Contains(err.Error(), "timeout_seconds") {
		t.Fatalf("invalid timeout: %v", err)
	}
	cfg.Speech.Command.TimeoutSeconds = 45
	cfg.Speech.Command.Cmd = nil
	if err := cfg.validate(); err == nil || !strings.Contains(err.Error(), "speech.command.cmd") {
		t.Fatalf("missing command: %v", err)
	}
}
