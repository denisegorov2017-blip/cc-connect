package max

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chenhg5/cc-connect/core"
)

type buttonMenu struct {
	reply     replyContext
	messageID string
	actions   []string
	labels    []string
	decision  bool
	expires   time.Time
}

func (p *Platform) sendBoundButtons(ctx context.Context, replyCtx any, content string, buttons [][]core.ButtonOption) error {
	// MAX recommends 10 characters in two-column rows, 20 in a single column.
	// Keep labels intact and give longer options (notably model IDs) a full row.
	var compactRows [][]core.ButtonOption
	for _, row := range buttons {
		columns := 2
		for _, button := range row {
			if len([]rune(button.Text)) > 10 {
				columns = 1
			}
		}
		for len(row) > 0 {
			n := min(columns, len(row))
			compactRows = append(compactRows, row[:n])
			row = row[n:]
		}
	}
	if len(compactRows) > 30 {
		return fmt.Errorf("max: keyboard exceeds 30 rows")
	}
	buttons = compactRows
	rctx, ok := replyCtx.(replyContext)
	if !ok || rctx.userID == "" {
		return fmt.Errorf("max: button menu requires a requester")
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Errorf("max: button menu nonce: %w", err)
	}
	nonce := hex.EncodeToString(random[:])
	menu := buttonMenu{reply: rctx, expires: time.Now().Add(30 * time.Minute)}
	rows := make([][]maxButton, 0, len(buttons))
	for _, row := range buttons {
		var out []maxButton
		for _, b := range row {
			menu.decision = menu.decision || strings.HasPrefix(b.Data, "perm:") || strings.HasPrefix(b.Data, "askq:")
			payload := fmt.Sprintf("ui:%s:%d", nonce, len(menu.actions))
			menu.actions = append(menu.actions, b.Data)
			menu.labels = append(menu.labels, b.Text)
			out = append(out, maxButton{Type: "callback", Text: b.Text, Payload: payload})
		}
		rows = append(rows, out)
	}
	chunks := splitMessage(content, 4000)
	for _, chunk := range chunks[:len(chunks)-1] {
		if err := p.sendText(ctx, rctx, chunk, nil); err != nil {
			return err
		}
	}
	body := &maxSendBody{Text: normalizeLineBreaks(chunks[len(chunks)-1]), Format: "markdown",
		Attachments: []maxOutAttachment{{Type: "inline_keyboard", Payload: maxKbPayload{Buttons: rows}}}}
	// Serialize publication with callbacks so a fast tap cannot beat registration.
	p.mu.Lock()
	defer p.mu.Unlock()
	if menu.decision {
		for key, existing := range p.buttonMenus {
			if existing.decision && existing.reply.userID == rctx.userID && existing.reply.chatID == rctx.chatID {
				delete(p.buttonMenus, key)
			}
		}
	}
	mid, err := p.postMessageID(ctx, rctx, body)
	if err != nil {
		return err
	}
	if mid == "" {
		return fmt.Errorf("max: sent button menu has no message ID")
	}
	menu.messageID = mid
	if p.buttonMenus == nil {
		p.buttonMenus = make(map[string]buttonMenu)
	}
	oldest := nonce
	oldestTime := menu.expires
	for key, existing := range p.buttonMenus {
		if time.Now().After(existing.expires) {
			delete(p.buttonMenus, key)
		} else if existing.expires.Before(oldestTime) {
			oldest, oldestTime = key, existing.expires
		}
	}
	if len(p.buttonMenus) >= 128 {
		delete(p.buttonMenus, oldest)
	}
	p.buttonMenus[nonce] = menu
	return nil
}

func (p *Platform) resolveButton(cb *maxCallback) (replyContext, string, bool) {
	parts := strings.Split(cb.Payload, ":")
	if len(parts) != 3 {
		return replyContext{}, "", false
	}
	index, err := strconv.Atoi(parts[2])
	if err != nil {
		return replyContext{}, "", false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	menu, ok := p.buttonMenus[parts[1]]
	if !ok || time.Now().After(menu.expires) || index < 0 || index >= len(menu.actions) ||
		menu.reply.userID != strconv.FormatInt(cb.User.UserID, 10) || cb.Message.Body.Mid != menu.messageID ||
		(cb.Message.Recipient.ChatID != 0 && strconv.FormatInt(cb.Message.Recipient.ChatID, 10) != menu.reply.chatID) {
		return replyContext{}, "", false
	}
	action := menu.actions[index]
	if action == "" {
		return replyContext{}, "", false
	}
	if index < len(menu.labels) {
		cb.buttonLabel = menu.labels[index]
	}
	if action != "cmd:/model" && action != "cmd:/reasoning" && action != "cmd:/list" && action != "cmd:/status" && action != "cmd:/workspace list" {
		if strings.HasPrefix(action, "perm:") || strings.HasPrefix(action, "askq:") {
			delete(p.buttonMenus, parts[1])
		} else {
			menu.actions[index] = ""
			p.buttonMenus[parts[1]] = menu
		}
	}
	rctx := menu.reply
	rctx.messageID = menu.messageID
	return rctx, action, true
}

func (p *Platform) answerButton(ctx context.Context, callbackID, text string) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	data, _ := json.Marshal(map[string]string{"notification": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiBase+"/answers", bytes.NewReader(data))
	if err != nil {
		slog.Debug("max: callback acknowledgment failed", "error", err)
		return
	}
	q := req.URL.Query()
	q.Set("callback_id", callbackID)
	req.URL.RawQuery = q.Encode()
	p.setAuth(req)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		slog.Debug("max: callback acknowledgment failed", "error", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Debug("max: callback acknowledgment rejected", "status", resp.StatusCode)
	}
}

func (p *Platform) SendPreviewStart(ctx context.Context, replyCtx any, content string) (any, error) {
	rctx, ok := replyCtx.(replyContext)
	if !ok {
		return nil, fmt.Errorf("max: unexpected replyCtx type %T", replyCtx)
	}
	mid, err := p.postMessageID(ctx, rctx, &maxSendBody{Text: content, Format: "markdown"})
	if err != nil {
		return nil, err
	}
	if mid == "" {
		return nil, fmt.Errorf("max: preview message has no message ID")
	}
	rctx.messageID = mid
	return rctx, nil
}

func (p *Platform) KeepPreviewOnFinish() bool { return true }

func (p *Platform) DeletePreviewMessage(ctx context.Context, handle any) error {
	rctx, ok := handle.(replyContext)
	if !ok || rctx.messageID == "" {
		return fmt.Errorf("max: preview message ID missing")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, p.apiBase+"/messages", nil)
	if err != nil {
		return err
	}
	q := req.URL.Query()
	q.Set("message_id", rctx.messageID)
	req.URL.RawQuery = q.Encode()
	p.setAuth(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("max: delete preview: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var result struct {
		Success bool `json:"success"`
	}
	if resp.StatusCode != http.StatusOK || json.Unmarshal(body, &result) != nil || !result.Success {
		return fmt.Errorf("max: delete preview rejected (HTTP %d)", resp.StatusCode)
	}
	return nil
}

var (
	_ core.PreviewStarter          = (*Platform)(nil)
	_ core.PreviewCleaner          = (*Platform)(nil)
	_ core.PreviewFinishPreference = (*Platform)(nil)
)
