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

func TestRussianButtonsAndMenuTitlesHaveTranslations(t *testing.T) {
	buttonOrTitle := regexp.MustCompile(`(?:btn_|_btn|_button|placeholder|^card_title_|^card_(?:back|prev|next)$)`)
	for key := range messages {
		if buttonOrTitle.MatchString(string(key)) {
			if translated, ok := russianMessages[key]; !ok || strings.TrimSpace(translated) == "" {
				t.Errorf("untranslated button/menu label: %s", key)
			}
		}
	}
}

type russianModeMenuAgent struct{ stubModelModeAgent }

func (*russianModeMenuAgent) PermissionModes() []PermissionModeInfo {
	return []PermissionModeInfo{
		{Key: "suggest", Name: "Suggest", Desc: "Read-only", NameKey: MsgModeSuggestName, DescKey: MsgModeSuggestDesc},
		{Key: "yolo", Name: "YOLO", Desc: "Dangerous", NameKey: MsgModeYoloName, DescKey: MsgModeYoloDesc},
	}
}

func TestRussianModeMenuLocalizesLabelsWithoutChangingPermissionValues(t *testing.T) {
	p := &stubInlineButtonPlatform{stubPlatformEngine: stubPlatformEngine{n: "inline"}}
	e := NewEngine("test", &russianModeMenuAgent{}, []Platform{p}, "", LangRussian)
	msg := &Message{SessionKey: "inline:chat:user", ReplyCtx: "ctx"}
	e.cmdMode(p, msg, nil)
	if p.buttonRows[0][0].Text != "Только чтение" || p.buttonRows[0][0].Data != "cmd:/mode suggest" {
		t.Fatalf("read-only action changed: %#v", p.buttonRows)
	}
	if p.buttonRows[0][1].Text != "Без ограничений" || p.buttonRows[0][1].Data != "cmd:/mode yolo" {
		t.Fatalf("unrestricted action changed: %#v", p.buttonRows)
	}
	if !strings.Contains(p.buttonContent, "ОПАСНО") {
		t.Fatal("unrestricted mode lost its danger description")
	}
}

func TestRussianReplyFooterUsesReadableLabels(t *testing.T) {
	e := NewEngine("test", &stubAgent{}, nil, "", LangRussian)
	e.replyFooterEnabled = true
	e.showContextIndicator = true
	agent := &stubFooterAgent{model: "gpt-6.1-terra", effort: "medium"}
	session := &controllableAgentSession{contextUsage: &ContextUsage{InputTokens: 1000, OutputTokens: 12, CachedInputTokens: 10, ContextWindow: 10000, UsedTokens: 2000}}
	footer := e.buildClaudeStatusLineFooter(agent, session, "")
	for _, want := range []string{"gpt-6.1-terra", "глубина:Средняя", "ответ 12", "вход", "контекст 20%"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer missing %q: %s", want, footer)
		}
	}
}
