# Голосовые MAX → Codex: рабочий контракт и доказательства

Состояние 16 ноября 2026: **production readiness не доказана**. Рабочая текстовая сборка `3f073734` не изменена. Цель создана по поручению пользователя; тестовый материал — уже отправленная запись, новые записи вместо неё не требуются.

## Исходный материал и границы

- Бот `552372453`, разрешённый пользователь `20201878`, диалог `567855475`. Исходная запись 03:24, около 1 секунды: `mid.0000000021d8c97301a1e1bd6790012e`.
- ID установлен из `link.message.mid` пользовательской пересылки 03:28:40: `mid.0000000021d8c97301a1e1c18de96f36`. Это связь с исходным голосовым, а не новая синтетическая запись.
- В MAX Desktop запись визуально присутствует. Кнопка встроенной расшифровки показала «Тест голоса». Это проверка клиентской возможности, **не** результат получения `transcription` ботом и **не** автоматический voice round trip.
- Разрешены локальные обратимые изменения и реальные проверки этого сообщения. Токен остаётся локальным; один receiver; Hermes MAX выключен; рабочие terra/medium и Beta сохраняются. Публикация, покупка STT и передача credentials новому сервису не разрешены.

## Проверено до реализации

1. Официальная схема MAX описывает `AudioAttachment.transcription` рядом с `payload`: строка nullable. У `payload` есть `url`/`token`. Непустая расшифровка может идти в Codex без скачивания аудио и STT; её наличие не гарантируется.
2. `GET /messages?chat_id=567855475&count=100` не содержит исходного audio. `GET /messages/{originalMid}` возвращает HTTP 200 с `code=empty.response.body`; `GET /messages?message_ids={originalMid}` — пустой массив. Повторное точечное чтение после клиентской расшифровки не дало тела сообщения. Проверка обоих MAX API hosts дала одинаковую ошибку; рабочий host остаётся документированным `platform-api2.max.ru`.
3. GET пересланного сообщения возвращает пустой `body.text`, без attachments; `link.message` содержит только исходные `mid`/`seq`. Из этих данных невозможно восстановить аудиобайты или расшифровку. Причина отсутствия медиа в Bot API пока не установлена; успешное чтение текстов и пересылки не доказывает исправность доставки audio.
4. Текущий adapter не читает `AudioAttachment.transcription`, использует недокументированный `/audios/{token}` при отсутствии URL и рано расходует in-memory dedup. Webhook подтверждается до durable обработки. Эти дефекты требуют отдельных отрицательных/повторных/recovery проверок; исправление только STT не устраняет проблему исходного материала.
5. `GET /subscriptions` вернул пустой массив. Работает один polling bridge. В текущем Hermes config нет публичного webhook URL; local 8646 listener и cloudflared/ngrok/caddy processes не обнаружены. Документация MAX рекомендует webhook и прямо не относит long polling к production. Нельзя объявить текущий polling production-ready или молча добавить новый внешний tunnel.
6. В установленном Hermes уже есть transcript-first реализация и регрессии: `plugins/max-platform/docs/transcript-first_RU.md`. Её документация отдельно фиксирует прежнее такое же расхождение GET для audio. Это полезный образец обработки, но не доказательство текущей доставки cc-connect. Hermes runtime и настройки не менялись.
7. Установленный Codex CLI **0.160.1** сгенерировал собственную JSON schema: `TurnStartParams` принимает `audio` с `url` и `localAudio` с `path`. Experimental realtime также есть, но не требуется для конечной голосовой записи. Схемы вне Git: `%LOCALAPPDATA%/cc-connect-max-analysis/codex-01601-schema`. Поддержка формата в протоколе ещё не доказывает доступность аудио выбранной модели или успешный запрос.
8. Реальный read-only `model/list` установленного app-server в 03:50 завершён без ошибок и cursor. Все 10 доступных моделей, включая terra, объявляют `inputModalities=[text,image]`, без audio. Это существенное ограничение: нельзя включить `localAudio` только потому, что variant присутствует в JSON schema. Проверочного model turn с исходной записью пока нет. Inventory завершил собственный процесс; основной bridge не менялся. Evidence вне Git: `candidate/native-audio-capabilities.json`.

## Следующие проверяемые этапы

1. Разрешить production архитектуру получения audio: документированный webhook, auth, durable admission/ограничения и исходная запись. Установить доступный способ получения исходного файла без ручной подмены текста и без неофициального userbot как незаявленного fallback. Если нужны внешний endpoint или права — сначала показать конкретный вариант и запросить недостающее решение.
2. Регрессия → исправление transcript-first в единственном MAX adapter: авторизация/дедуп до обработки, непустой transcript без download/STT, подпись и другие вложения сохранены, пустой/null/invalid transcript сохраняет аудиопуть. Не превращать произнесённую строку в обход admin/approval политики.
3. Проверить нативный Codex audio/localAudio на исходном файле и выбранной модели. Только подтверждённый путь включать в канонический optional-capability контракт core/agent; exec не должен притворяться поддерживающим аудио. Не подключать платный STT и не извлекать auth для неофициальных API.
4. Ошибки download/размера/формата/timeout и повторная доставка: понятный русский результат, секреты/подписанные URL не логируются, повтор не запускает второй turn, recovery не теряет уже принятую запись. Хранение и cleanup ограничены и принадлежат одному владельцу.
5. Измерить на этой же записи получение → transcript/audio processing → Codex first/completion → MAX delivery. Сначала serial baseline, затем изменения только измеренного bottleneck; сравнивать одинаковый материал. Готовый transcript предпочтителен как устранение ненужной работы, но заявление ускорения требует живых замеров.
6. Регрессии, CUJ через ReceiveMessage, vet, owning gates; точный commit/archive build; safe install и live end-to-end именно на исходной записи; restart/rollback и итоговая матрица. Старое исключение для локального коммита не превращает непрошедший release gate в успешный.

## Локальный checkpoint transcript-first

- Регрессия `TestNativeTranscriptSkipsAudioDownloadAndPreservesCaption` сначала воспроизвела дефект: URL уже расшифрованного audio скачивался (HTTP 410), текст оставался только «Подпись», transcript терялся. RED сохранён вне Git: `cc-connect-max-analysis/voice-transcript-red.log`.
- Добавлены чтение nullable/invalid optional `transcription` без отказа всего JSON, выделение готовых audio-транскриптов перед download и сохранение caption/порядка/других вложений. Поле `FromVoice` сохраняет происхождение сообщения. Контроль пользователя и дедуп остаются до обработки. Лог содержит только фиксированный источник `max` и количество, без текста/URL.
- `TestNativeTranscriptFallbackRetainsMediaAndOriginalInput` проверяет null/blank/invalid и нетронутый вход; `TestNativeTranscriptHonorsUserAllowList` — запрет чужому пользователю. Полный `go test ./platform/max -count 1 -timeout 60s`: 8,829 с, успешно; `go vet ./platform/max` успешно. Это локальная проверка, не доказательство live transcript/MAX delivery.
- Диагностика одноразового чтения `/updates` с ранее наблюдавшимся marker после native stop отклонена automatic approval review: `blocked by policy`, подробная причина отсутствует. Запрос не исполнился, raw queue не получена. Повтор альтернативным путём не делался. Проверен оставшийся старый child и отсутствующий parent; child остановлен по точному PID/path/creation и native task снова запущен. Рабочая сборка остаётся прежней.
- Попытка переключить будущий режим Plan через штатный owner helper получила `The task has an active turn; wait for an idle boundary`; selector не изменён. Текущий turn остаётся Default. Архитектурная production-приёмка не заявлена; выполнено независимое ограниченное исправление известного формата transcript-first.

## Источники

- [MAX Message](https://dev.max.ru/docs-api/objects/Message).
- [Официальная схема MAX](https://github.com/max-messenger/api-schema/blob/main/schema.yaml).
- [MAX Updates](https://dev.max.ru/docs-api/objects/Update/).
- [GET messages](https://dev.max.ru/docs-api/methods/GET/messages).
- Локальная schema установленного Codex 0.160.1; текущие исходники `core/speech.go`, `core/engine.go`, `agent/codex/appserver_session.go`, `platform/max/max.go`.
