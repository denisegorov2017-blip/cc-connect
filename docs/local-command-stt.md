# Local command speech recognition

`provider = "command"` connects an already installed recognizer without an HTTP
service or cloud credentials. Configure an executable and an argument array:

```toml
[speech]
enabled = true
provider = "command"
language = "ru"

[speech.command]
cmd = ['C:\Python\python.exe', 'C:\recognizer\adapter.py']
timeout_seconds = 45
```

Restart cc-connect after changing speech configuration. Speech settings are not
hot-reloaded. The command is started directly, without shell expansion. Audio
bytes arrive on stdin; `CC_CONNECT_STT_FORMAT` and `CC_CONNECT_STT_LANGUAGE` carry
the format and language hint. Stdout must contain exactly one UTF-8 JSON object
with a string `text` property, for example `{"text":"recognized speech"}`. An
empty string follows the existing "no speech recognized" path. Use a nonzero
exit status on failure. Stderr is discarded and never reflected to the chat.

One command runs at a time per provider instance. Waiting and execution share
the configured timeout: default 45 seconds, maximum 120 seconds. The direct
child is terminated on timeout/cancellation; trusted commands must manage any
descendants they create. Input is limited to 25 MiB and stdout to 1 MiB. These
limits do not implement a durable incoming-message queue or crash recovery.

Existing format conversion still applies. For formats such as Ogg/Opus,
cc-connect currently requires ffmpeg on its own process PATH.

## Reusing Hermes

The optional [adapter](examples/hermes_local_stt.py) calls the installed Hermes
`transcribe_audio_local_fallback` function. Run it with the Python environment
that already contains Hermes and faster-whisper:

```toml
[speech.command]
cmd = ['C:\Hermes\venv\Scripts\python.exe', 'C:\cc-connect\docs\examples\hermes_local_stt.py', '--hermes-dir', 'C:\Hermes\hermes-agent', '--hermes-home', 'C:\Hermes\profile', '--model-dir', 'C:\models\faster-whisper-base']
timeout_seconds = 45
```

`--model-dir` must name an existing local model directory containing `model.bin`.
The adapter enables offline mode and uses the explicit local-only Hermes entry
point; it does not install packages, download models, change Hermes settings,
or select a cloud fallback. Hermes retains ownership of recognition settings,
including its language and silence/hallucination filtering.

Audio is normally processed in an automatically removed temporary directory.
For a deliberately recorded diagnostic, `--evidence-dir C:\diagnostics\voice`
retains the input audio and a JSON report with its SHA256, transcript, byte count,
and recognition-call duration. Audio is retained before model loading and
recognition; on failure the audio remains, but no success report is written.
This diagnostic copy is not a durable webhook queue or automatic replay.
Keep that directory outside the repository.
This timing excludes subprocess startup and the subsequent agent turn.

The example transport tests mock only the Hermes recognizer and do not establish
live voice recognition. Run them with:

```text
python -m unittest discover -s docs/examples -p test_hermes_local_stt.py -v
```
