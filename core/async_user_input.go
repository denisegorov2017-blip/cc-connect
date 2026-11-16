package core

import (
	"crypto/rand"
	"fmt"
	"strconv"
	"strings"
)

type pendingUserInput struct {
	session    *Session
	id         string
	questions  []UserQuestion
	answers    []string
	current    int
	responding bool
}

func (e *Engine) showAsyncUserInput(state *interactiveState, p Platform, replyCtx any, event Event, origin ...*Session) {
	if event.RequestID == "" || len(event.Questions) == 0 {
		return
	}
	pending := &pendingUserInput{
		id:        "async_" + rand.Text(),
		questions: event.Questions,
		answers:   make([]string, len(event.Questions)),
	}
	if len(origin) > 0 {
		pending.session = origin[0]
	}
	state.mu.Lock()
	state.pendingUserInput = pending
	state.mu.Unlock()
	e.sendAskQuestionPrompt(p, replyCtx, pending.questions, 0, pending.id)
}

// Unlike a permission request, an async question neither stops the event reader
// nor waits on RespondPermission. Its answer is new user input to the same thread.
func (e *Engine) handleAsyncUserInput(p Platform, msg *Message, content, key string, sessions *SessionManager) (string, bool) {
	parts := strings.Split(content, ":")
	callback := len(parts) == 4 && parts[0] == "askq" && strings.HasPrefix(parts[1], "async_")
	if strings.HasPrefix(content, "askq:") && !callback {
		return content, false // blocking request; owned by handlePendingPermission
	}
	e.interactiveMu.Lock()
	state := e.interactiveStates[key]
	e.interactiveMu.Unlock()
	if state == nil {
		return content, callback
	}
	state.mu.Lock()
	pending := state.pendingUserInput
	if pending == nil || (!callback && state.pending != nil) {
		state.mu.Unlock()
		return content, callback
	}
	if pending.responding || strings.TrimSpace(content) == "" || len(msg.Images) > 0 || len(msg.Files) > 0 {
		state.mu.Unlock()
		return content, callback
	}
	index := pending.current
	question := pending.questions[index]
	if callback {
		qIdx, qErr := strconv.Atoi(parts[2])
		option, optErr := strconv.Atoi(parts[3])
		if parts[1] != pending.id || qErr != nil || qIdx != index || optErr != nil || option < 1 || option > len(question.Options) {
			state.mu.Unlock()
			return content, true
		}
		content = strconv.Itoa(option)
	}
	answer := e.resolveAskQuestionAnswer(question, content)
	pending.answers[index] = answer
	if index+1 < len(pending.questions) {
		pending.current++
		next := pending.current
		state.mu.Unlock()
		e.reply(p, msg.ReplyCtx, fmt.Sprintf("✅ %s: **%s**", question.Question, answer))
		e.sendAskQuestionPrompt(p, msg.ReplyCtx, pending.questions, next, pending.id)
		return content, true
	}
	pending.responding = true
	agentSession := state.agentSession
	var lines []string
	for i, q := range pending.questions {
		lines = append(lines, q.Question+": "+pending.answers[i])
	}
	prompt := strings.Join(lines, "\n")
	state.mu.Unlock()

	steered := false
	var err error
	session := pending.session
	if session == nil {
		session = sessions.GetOrCreateActive(msg.SessionKey)
	}
	if sessions.ActiveSessionID(msg.SessionKey) != session.ID {
		return content, true // origin was switched/reset while the answer was being collected
	}
	if sender, ok := agentSession.(UserInputSteerer); ok {
		steered, err = sender.SteerUserInput(prompt)
	}
	state.mu.Lock()
	if err != nil {
		pending.responding = false
	} else if state.pendingUserInput == pending {
		state.pendingUserInput = nil
	}
	state.mu.Unlock()
	if err != nil {
		e.reply(p, msg.ReplyCtx, fmt.Sprintf(e.i18n.T(MsgError), err))
		return content, true
	}
	e.reply(p, msg.ReplyCtx, fmt.Sprintf("✅ %s: **%s**", question.Question, answer))
	if steered {
		session.AddHistory("user", prompt)
		sessions.Save()
		return prompt, true
	}
	// A completed turn uses the ordinary message path, including its busy lock,
	// queue, history persistence and foreground event-reader ownership.
	msg.IsPermissionResponse = false // validated answer is now user input, not a stale permission click
	msg.questionSession = session
	return prompt, false
}
