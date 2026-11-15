package core

// Russian covers the chat menus and their immediate feedback. Other messages
// use the existing English fallback until their translations are added.
var russianMessages = map[MsgKey]string{
	MsgMenuNew:                    "Новый чат",
	MsgMenuSessions:               "Чаты",
	MsgMenuModel:                  "Модель",
	MsgMenuReasoning:              "Глубина",
	MsgMenuProjects:               "Проекты",
	MsgCardTitleStatus:            "Статус",
	MsgBuiltinCmdStop:             "Стоп",
	MsgCardTitleLanguage:          "Язык",
	MsgCardTitleReasoning:         "Глубина рассуждений",
	MsgStarting:                   "⏳ Обрабатываю…",
	MsgHelp:                       "📖 Команды бота\n\n/new [имя] — новый чат\n/list — список чатов\n/switch <номер> — открыть чат\n/history — история\n/model — выбор модели\n/reasoning — глубина рассуждений\n/workspace list — проекты\n/status — состояние подключения\n/stop — остановить задачу\n/skills — доступные навыки\n/doctor — диагностика\n/lang — язык интерфейса\n\nМожно пользоваться кнопками под сообщением. Если отправляете файл, дождитесь его доставки и затем напишите задание.",
	MsgWelcome:                    "👋 Бот подключён к **%s**.\n\nНапишите сообщение агенту или выберите действие кнопкой. Справка: /help.",
	MsgLangChanged:                "🌐 Язык переключён: **%s**.",
	MsgLangCurrent:                "🌐 Текущий язык: **%s**\n\nСмена: /lang <en|zh|zh-TW|ja|es|ru|auto>",
	MsgLangInvalid:                "Неизвестный язык. Доступны: `en`, `zh`, `zh-TW`, `ja`, `es`, `ru`, `auto`.",
	MsgLangSelectPlaceholder:      "Выберите язык",
	MsgModelSelectPlaceholder:     "Выберите модель",
	MsgReasoningSelectPlaceholder: "Выберите глубину",
	MsgModelCurrent:               "Текущая модель: %s",
	MsgModelDefault:               "Модель: по умолчанию для агента\n",
	MsgModelListTitle:             "Доступные модели:\n",
	MsgModelUsage:                 "Нажмите кнопку модели или отправьте `/model switch <имя>`.",
	MsgModelChanged:               "✅ Выбрана модель `%s`. Она будет использоваться в текущей и новых сессиях.",
	MsgModelChangeFailed:          "❌ Не удалось сменить модель: %v",
	MsgReasoningCurrent:           "Глубина рассуждений: %s",
	MsgReasoningDefault:           "Глубина рассуждений: по умолчанию агента\n",
	MsgReasoningListTitle:         "Доступные уровни:\n",
	MsgReasoningUsage:             "Выберите кнопку или отправьте `/reasoning <low|medium|high|xhigh>`.",
	MsgReasoningChanged:           "✅ Глубина изменена: `%s`. Настройка применяется при следующем запуске сессии.",
	MsgReasoningNotSupported:      "Этот агент не поддерживает выбор глубины рассуждений.",
	MsgNewSessionCreated:          "✅ Создан новый чат",
	MsgNewSessionCreatedName:      "✅ Создан новый чат: **%s**",
	MsgListTitle:                  "**Чаты %s** (%d)\n\n",
	MsgListTitlePaged:             "**Чаты %s** (%d) · Страница %d/%d\n\n",
	MsgListEmpty:                  "В этом проекте пока нет чатов.",
	MsgListSwitchHint:             "\nВыберите чат кнопкой или отправьте `/switch <номер>`.",
	MsgListError:                  "❌ Не удалось получить чаты: %v",
	MsgSwitchSuccess:              "✅ Открыт чат: %s (%s, сообщений: %d)",
	MsgSwitchNoMatch:              "❌ Чат %q не найден",
	MsgWsListEmpty:                "Проект ещё не выбран.",
	MsgWsBindSuccess:              "✅ Выбран проект: `%s`",
	MsgWsBindNotFound:             "Проект не найден: `%s`",
	MsgWsResolutionError:          "❌ Не удалось открыть проект: %v",
	MsgExecutionStopped:           "⏹ Выполнение остановлено.",
	MsgNoExecution:                "Сейчас нет выполняемой задачи.",
	MsgSessionCancelled:           "Сессия сброшена. Можно отправить новое задание.",
	MsgMessageQueued:              "📬 Запрос принят в очередь. Выполню после текущей задачи.",
	MsgQueueFull:                  "📬 Очередь заполнена: %d запросов. Дождитесь завершения текущих задач.",
	MsgPermBtnAllow:               "Разрешить",
	MsgPermBtnDeny:                "Отклонить",
	MsgPermBtnAllowAll:            "Всё на эту сессию",
	MsgPermCardTitle:              "Запрос разрешения",
	MsgPermCardBody:               "Агент запрашивает **%s**:\n\n```\n%s\n```",
	MsgPermCardNote:               "Если кнопки не работают, ответьте: allow / deny / allow all",
	MsgPermissionPrompt:           "⚠️ **Запрос разрешения**\n\nАгент запрашивает **%s**:\n\n```\n%s\n```\n\nВыберите кнопку. «Всё на эту сессию» автоматически разрешит последующие запросы до завершения этой сессии. Текстовый ответ: allow / deny / allow all.",
	MsgPermissionAllowed:          "✅ Разрешено. Продолжаю…",
	MsgPermissionApproveAll:       "✅ Все последующие запросы в этой сессии будут разрешаться автоматически.",
	MsgPermissionDenied:           "❌ Отклонено. Агент остановит это действие.",
	MsgPermissionHint:             "⚠️ Ожидаю разрешение. Нажмите кнопку или ответьте: allow / deny / allow all.",
	MsgAskQuestionTitle:           "Уточнение",
	MsgAskQuestionNote:            "Выберите кнопку, отправьте номер варианта или напишите свой ответ.",
	MsgAskQuestionNoteMulti:       "Можно отправить несколько номеров через запятую, например 1,3, или свой ответ.",
	MsgAskQuestionMulti:           " (можно выбрать несколько вариантов через запятую)",
	MsgAskQuestionPrompt:          "❓ **%s**\n\n%s\n\nВыберите номер варианта или напишите ответ.",
	MsgAskQuestionAnswered:        "Ответ",
	MsgEnabledShort:               "ВКЛ",
	MsgDisabledShort:              "ВЫКЛ",
	MsgUntitled:                   "Без названия",
	MsgStatusTitle:                "Статус cc-connect\n\nПроект: %s\nАгент: %s\nРабочая папка: %s\nПлатформы: %s\nВремя работы: %s\nЯзык: %s\n%s%s%s%s%s%s",
	MsgStatusMode:                 "Режим: %s\n",
	MsgStatusSession:              "Чат: %s (сообщений: %d)\n",
	MsgStatusCron:                 "Расписания: %d (включено: %d)\n",
	MsgStatusThinkingMessages:     "Показывать ход рассуждений: %s\n",
	MsgStatusToolMessages:         "Ход работы инструментов: %s\n",
	MsgStatusSessionKey:           "Ключ чата: `%s`\n",
	MsgStatusAgentSID:             "Сессия агента: `%s`\n",
	MsgStatusUserID:               "ID пользователя: `%s`\n",
}

func (i *I18n) reasoningEffortLabel(effort string) string {
	if i.CurrentLang() == LangRussian {
		switch effort {
		case "low":
			return "Низкая"
		case "medium":
			return "Средняя"
		case "high":
			return "Высокая"
		case "xhigh":
			return "Максимум"
		}
	}
	return effort
}
