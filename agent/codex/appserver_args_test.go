package codex

import (
	"reflect"
	"testing"
)

func TestAppServerArgsKeepConfigOverridesInOneParserScope(t *testing.T) {
	s := &appServerSession{
		cliExtraArgs: []string{"-c", "features.default_mode_request_user_input=true", "--profile", "max"},
		model:        "gpt-6.1-sol", effort: "low", modelProvider: "local", baseURL: "http://localhost:1234/v1", url: "stdio://",
	}
	before := append([]string(nil), s.cliExtraArgs...)
	got := s.appServerArgs()
	want := []string{"-c", "features.default_mode_request_user_input=true", "--profile", "max", "-c", `model="gpt-6.1-sol"`, "-c", `model_reasoning_effort="low"`, "-c", `model_provider="local"`, "-c", `openai_base_url="http://localhost:1234/v1"`, "app-server", "--listen", "stdio://"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("config overrides must precede subcommand together: got %q; want %q", got, want)
	}
	if !reflect.DeepEqual(s.cliExtraArgs, before) {
		t.Fatal("building arguments mutated configured command")
	}
}

func TestAppServerArgsWithoutOverrides(t *testing.T) {
	if got := (&appServerSession{}).appServerArgs(); !reflect.DeepEqual(got, []string{"app-server"}) {
		t.Fatalf("unexpected defaults: %q", got)
	}
}
