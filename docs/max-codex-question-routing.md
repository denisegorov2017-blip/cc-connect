# Codex questions in MAX

Use the Codex `app_server` backend. Two independent protocol paths are supported:

- Blocking `item/tool/requestUserInput`: answer the original JSON-RPC request,
  keyed by its question IDs. Child-thread and stale-turn requests are not shown.
- Async `agentMessage` items with `delivery: "async"`: display their structured
  questions and use `turn/steer` with the original thread and expected turn.
  After the turn ends, submit the validated answer through the normal message
  path, pinned to the original cc-connect session.

The async implementation is adapted from upstream cc-connect commit
`0d4376e6747920ff425d921970e2ae792039d311` (PR #2048), with random per-question
tokens, blocking/async callback separation, and session pinning. Upstream commit
`0a4b4617264d582ac58e33aee534fc15641605e5` also documents the stale-option problem.

MAX keyboards bind random payloads to the requester, bot message and dialog.
Question callbacks additionally bind the request token, question index and option.
Repeated/expired clicks are discarded. Restarting the bridge or switching/resetting
the source session invalidates outstanding buttons; they cannot answer a new chat.
Questions are displayed sequentially. The user can select a button, send an option
number, or type another answer. A consumed keyboard is removed with an explicit
empty `attachments` array, as required by the MAX edit API.

An ordinary model paragraph containing choices is not a structured question.
For consistent presentation, configure `append_system_prompt` to prefer the
available structured question tool and ask only for information missing from
the user's message. Do not automatically choose an option for the user.

Local checks: `go test ./core ./agent/codex ./platform/max ./integration/maxcodex`
and `go test ./core -run TestCUJ`.

The opt-in `integration/maxcodex` live test requires an isolated work directory,
the existing bot token in the process environment, and exclusive polling ownership.
It uses the real Codex process and MAX API; it never creates a payment/document.
Its supervisor must bound execution and restore the ordinary bridge on exit.
