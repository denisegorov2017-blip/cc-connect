package core

// Translation catalog is populated separately; missing messages fall back to English.
var russianMessages = map[MsgKey]string{}

func (i *I18n) reasoningEffortLabel(effort string) string { return effort }
