# Context efficiency checkpoint

Keep the existing compact token footer. No separate statistics panel, model change,
global instruction rewrite, or automatic history compaction is part of this change.

Necessary fix: app-server result events previously emitted zero input/output usage.
Forward actual last-request usage, preserve cache-write counts, and log turn/session
cumulative counts separately. Duplicate usage notifications must not be added twice;
foreign/stale updates and unknown resume baselines must not fabricate statistics.
Runtime-reported context capacity takes precedence over the legacy fixed-window guess.

Retained MAX question probe: last request 34,833 input (34,560 cached), 13 output,
34,846 total. Complete turn: 104,323 input (69,120 cached; 35,203 uncached), 72 output,
104,395 total. Current context and cumulative consumption are different quantities.
The token-meter test replays the native journal without calling a model.

No significant context reduction is claimed from these telemetry fixes. Existing
menus, option validation, callback routing and deduplication already execute in Go.
Keep them outside model inference. The bridge sends new user input to a persistent
thread, rather than replaying its own entire history on each message.

The probe's first request already used 34,716 input tokens. Stable runtime instructions,
the skill catalog and tools account for substantial startup context. Arbitrarily
removing them or changing the prefix would alter behavior/cache reuse. Large rollout
JSON records containing native images are not equivalent to base64 text in a prompt.

Primary references checked 2026-11-16:
- https://learn.chatgpt.com/docs/app-server
- https://developers.openai.com/api/docs/guides/prompt-caching
- https://developers.openai.com/api/docs/guides/compaction
- Codex rust-v0.160.1 ThreadTokenUsageUpdatedNotification schema.
