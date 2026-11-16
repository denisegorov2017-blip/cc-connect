package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const maxCommandSpeechOutput = 1024 * 1024

// CommandSTT sends audio bytes to a configured executable's stdin, without a
// shell. The executable returns one JSON object {"text":"recognized speech"}.
// Format and language are supplied as CC_CONNECT_STT_FORMAT/LANGUAGE environment
// variables. Audio and process stderr are never included in errors.
type CommandSTT struct {
	command []string
	timeout time.Duration
	slots   chan struct{}
}

func NewCommandSTT(command []string, timeout time.Duration) *CommandSTT {
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	if timeout > 2*time.Minute {
		timeout = 2 * time.Minute
	}
	return &CommandSTT{command: append([]string(nil), command...), timeout: timeout, slots: make(chan struct{}, 1)}
}

type speechOutput struct {
	bytes.Buffer
	exceeded bool
}

func (b *speechOutput) Write(p []byte) (int, error) {
	if len(p) > maxCommandSpeechOutput-b.Len() {
		b.exceeded = true
		return 0, errors.New("speech command output limit exceeded")
	}
	return b.Buffer.Write(p)
}

func (s *CommandSTT) Transcribe(ctx context.Context, audio []byte, format, lang string) (string, error) {
	if len(s.command) == 0 || strings.TrimSpace(s.command[0]) == "" {
		return "", errors.New("speech command is not configured")
	}
	if len(audio) == 0 || len(audio) > 25*1024*1024 {
		return "", errors.New("speech command audio size is invalid")
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return "", fmt.Errorf("speech command: %w", ctx.Err())
	}
	cmd := exec.CommandContext(ctx, s.command[0], s.command[1:]...)
	configureSpeechCommand(cmd)
	cmd.WaitDelay = time.Second
	cmd.Env = MergeEnv(os.Environ(), []string{"CC_CONNECT_STT_FORMAT=" + format, "CC_CONNECT_STT_LANGUAGE=" + lang})
	cmd.Stdin = bytes.NewReader(audio)
	var output speechOutput
	cmd.Stdout = &output
	cmd.Stderr = io.Discard
	err := cmd.Run()
	if ctx.Err() != nil {
		return "", fmt.Errorf("speech command: %w", ctx.Err())
	}
	if output.exceeded {
		return "", errors.New("speech command output limit exceeded")
	}
	if err != nil {
		return "", fmt.Errorf("speech command failed: %w", err)
	}
	var result struct {
		Text *string `json:"text"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil || result.Text == nil {
		return "", errors.New("speech command returned invalid JSON text")
	}
	return strings.TrimSpace(*result.Text), nil
}
