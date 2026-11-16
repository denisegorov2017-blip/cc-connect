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

## Measured question-wait optimization (2026-11-16)

Codex CLI 0.160.1 replaces global `-c` overrides when another `-c` appears
after `app-server`. Previously the bridge put configured feature flags before
the subcommand and generated model/provider overrides after it. A native
`config/read` probe confirmed that `default_mode_request_user_input=true`
disappeared. The adapter now places all overrides before the subcommand.
No model catalog, global instructions, permissions, or history are rewritten.

For callers that have enabled this native feature, prefer `request_user_input`
for clarification and use `request_user_input_async` when the native tool is
unavailable. The existing app-server request handler waits for the answer in
Go and returns it to the original RPC/thread. No model polling or intermediate
acknowledgment is needed. The async fallback and its session/deduplication
protections remain supported. Existing threads retain their earlier prompt;
a changed first-message preamble is applied to new threads only.

Two real-inference comparisons used Codex 0.160.1, `gpt-6.1-sol`, low effort,
Default mode, a local MAX HTTP fixture, real Engine/SessionManager/MAX adapter,
the same question and final answer, and a controlled 12-second button delay.
These are bridge integration measurements, not a new test on the live MAX service.

| Measurement | Run 1 before | Run 1 after | Run 2 before | Run 2 after |
|---|---:|---:|---:|---:|
| Model requests | 3 | 2 | 3 | 2 |
| Cumulative input | 104,659 | 69,744 | 104,662 | 69,744 |
| Cached input (included above) | 76,544 | 34,688 | 69,504 | 34,688 |
| Output | 72 | 86 | 99 | 86 |
| Last-request input | 34,945 | 34,921 | 34,948 | 34,921 |

Both runs used the concise preference below. Native rollout token-counter
steps confirmed the request
counts; the after runs called `request_user_input`, and the before runs used
the asynchronous question path. Both produced `MAX-BUTTONS-OK:Расход` after
the selected Russian button in the original isolated thread.

Run 1 reduced cumulative input by 34,915 tokens (33.36%); run 2 by 34,918 (33.36%). This is one
fewer inference over the context, not a 33% smaller retained context window.
Cache warmth varied: uncached input was 28,115 before and 35,056 after in the
acceptance pair. Therefore neither proportional billing savings nor a general
latency improvement is established. No history deletion or compaction was used.

Concise preference replacing only the earlier async-first sentence:

> Для уточняющих вопросов с вариантами ответа используйте request_user_input, когда он доступен; иначе request_user_input_async.

Reproduce with the opt-in `integration/maxcodex/context_benchmark_test.go`:
set `CC_CONTEXT_BENCHMARK=1`, an absolute isolated `CC_CONTEXT_BENCHMARK_ROOT`,
`CC_CONTEXT_BASE_CONFIG` to the existing config, `CC_CODEX_BINARY` to native
Codex, `CC_CONTEXT_SOURCE` to the source identity, and `CC_CONTEXT_VARIANT`
to `before` or `after`. Run
`go test ./integration/maxcodex -run '^TestContextQuestionBenchmark$' -count=1 -v -timeout 140s`.
Use the same benchmark source on baseline `c1280ea2` and the fixed source,
with the same root/model/effort. Never overwrite an existing run; use a new root
for another pair. Tests never poll the real bot and are skipped by default.
The saved result records prompt/source hashes and actual native counters.

Hooks were considered but are not needed for this fix. cc-connect hooks are
lifecycle notifications, not request/response interceptors. Codex hooks may
add developer context or force another continuation, so adding one without a
measured redundant operation could increase consumption instead.
