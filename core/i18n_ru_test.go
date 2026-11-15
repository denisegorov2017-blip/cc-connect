package core

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestConfiguredRussianLanguageSurvivesStartup(t *testing.T) {
	for _, value := range []string{"ru", "russian"} {
		if NewI18n(ParseLanguage(value)).T(MsgMenuNew) != "Новый чат" {
			t.Fatalf("config language %q fell back to auto/English", value)
		}
	}
	for value, want := range map[string]Language{"en": LangEnglish, "zh": LangChinese, "zh-TW": LangTraditionalChinese, "ja": LangJapanese, "es": LangSpanish, "": LangAuto} {
		if got := ParseLanguage(value); got != want {
			t.Errorf("existing locale %q changed: %q", value, got)
		}
	}
}

func TestRussianMenuTranslationsPreserveFormatting(t *testing.T) {
	verbs := regexp.MustCompile(`%(?:\[[0-9]+\])?[-+# .0-9]*[a-zA-Z%]`)
	i := NewI18n(LangRussian)
	for key, translated := range russianMessages {
		english, ok := messages[key][LangEnglish]
		if !ok {
			t.Errorf("Russian translation has no source: %s", key)
			continue
		}
		if !reflect.DeepEqual(verbs.FindAllString(english, -1), verbs.FindAllString(translated, -1)) {
			t.Errorf("format parameters differ for %s: English=%q Russian=%q", key, english, translated)
		}
		if i.T(key) != translated {
			t.Errorf("Russian locale did not use %s", key)
		}
	}
	if DetectLanguage("Покажи чаты") != LangRussian {
		t.Fatal("Russian auto-detection failed")
	}
}

func TestRussianMenusKeepStableCommands(t *testing.T) {
	p := &stubInlineButtonPlatform{stubPlatformEngine: stubPlatformEngine{n: "inline"}}
	e := NewEngine("test", &stubModelModeAgent{}, []Platform{p}, "", LangEnglish)
	msg := &Message{SessionKey: "inline:chat:user", ReplyCtx: "ctx"}
	if !e.handleCommand(p, msg, "/lang ru") || e.i18n.CurrentLang() != LangRussian {
		t.Fatal("language command did not select Russian")
	}
	e.cmdStart(p, msg)
	if p.buttonRows[0][0].Text != "Новый чат" || p.buttonRows[1][1].Text != "Глубина" {
		t.Fatalf("main menu not translated: %#v", p.buttonRows)
	}
	for _, row := range p.buttonRows {
		for _, button := range row {
			if !strings.HasPrefix(button.Data, "cmd:/") || len([]rune(button.Text)) > 10 {
				t.Errorf("invalid compact menu button: %#v", button)
			}
		}
	}
	e.cmdReasoning(p, msg, nil)
	if p.buttonRows[0][1].Text != "Средняя" || p.buttonRows[0][1].Data != "cmd:/reasoning medium" {
		t.Fatalf("translated effort changed command: %#v", p.buttonRows)
	}
}
