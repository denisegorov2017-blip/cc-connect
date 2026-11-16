package codex

import (
	"github.com/chenhg5/cc-connect/core"
)

// The native cumulative counters are the source of truth. Duplicated usage
// notifications and context-only estimates must not be charged a second time.
type appServerTokenMeter struct {
	turnID                  string
	total, last, base, turn *core.TokenCounts
}

func copyTokenCounts(c *core.TokenCounts) *core.TokenCounts {
	if c == nil {
		return nil
	}
	copy := *c
	return &copy
}

func (m *appServerTokenMeter) begin(turnID string) {
	if turnID == "" || m.turnID == turnID {
		return
	}
	m.turnID, m.base, m.turn, m.last = turnID, copyTokenCounts(m.total), nil, nil
}

func tokenCounts(u codexTokenUsage) *core.TokenCounts {
	c := &core.TokenCounts{InputTokens: u.InputTokens, CachedInputTokens: u.CachedInputTokens, CacheWriteInputTokens: u.CacheWriteInputTokens, OutputTokens: u.OutputTokens, ReasoningOutputTokens: u.ReasoningOutputTokens, TotalTokens: u.TotalTokens}
	if !validTokenCounts(c) || c.TotalTokens == 0 {
		return nil
	}
	return c
}

func validTokenCounts(c *core.TokenCounts) bool {
	return c != nil && c.InputTokens >= 0 && c.CachedInputTokens >= 0 && c.CacheWriteInputTokens >= 0 && c.OutputTokens >= 0 && c.ReasoningOutputTokens >= 0 && c.TotalTokens == c.InputTokens+c.OutputTokens && c.CachedInputTokens <= c.InputTokens && c.CacheWriteInputTokens <= c.InputTokens && c.ReasoningOutputTokens <= c.OutputTokens
}

func subtractTokenCounts(a, b *core.TokenCounts) *core.TokenCounts {
	c := &core.TokenCounts{InputTokens: a.InputTokens - b.InputTokens, CachedInputTokens: a.CachedInputTokens - b.CachedInputTokens, CacheWriteInputTokens: a.CacheWriteInputTokens - b.CacheWriteInputTokens, OutputTokens: a.OutputTokens - b.OutputTokens, ReasoningOutputTokens: a.ReasoningOutputTokens - b.ReasoningOutputTokens, TotalTokens: a.TotalTokens - b.TotalTokens}
	if !validTokenCounts(c) {
		return nil
	}
	return c
}

func (m *appServerTokenMeter) observe(n appServerThreadTokenUsageNotification) {
	total := tokenCounts(n.TokenUsage.Total)
	if total == nil || (m.turnID != "" && n.TurnID != "" && n.TurnID != m.turnID) {
		return
	}
	if m.total != nil && *m.total == *total {
		return
	}
	if m.total != nil && subtractTokenCounts(total, m.total) == nil {
		m.base, m.turn, m.last = nil, nil, nil // counter discontinuity: don't fabricate a delta
		m.total = total
		return
	}
	m.total = total
	m.last = tokenCounts(n.TokenUsage.Last)
	if m.turnID == "" {
		return
	} // resume-time totals seed the next turn's baseline
	if m.base == nil && m.last != nil && *m.last == *total {
		m.base = subtractTokenCounts(total, m.last)
	}
	if m.base != nil {
		m.turn = subtractTokenCounts(total, m.base)
	}
}

func (s *appServerSession) GetTokenUsageSnapshot() core.TokenUsageSnapshot {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return core.TokenUsageSnapshot{ThreadID: s.CurrentSessionID(), TurnID: s.tokenMeter.turnID, LastRequest: copyTokenCounts(s.tokenMeter.last), Turn: copyTokenCounts(s.tokenMeter.turn), Session: copyTokenCounts(s.tokenMeter.total)}
}
