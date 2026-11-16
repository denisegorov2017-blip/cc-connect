package core

// TokenCounts contains provider-reported usage, never a character estimate.
// Cached input is included in InputTokens; reasoning is included in OutputTokens.
type TokenCounts struct {
	InputTokens, CachedInputTokens, CacheWriteInputTokens int
	OutputTokens, ReasoningOutputTokens, TotalTokens      int
}

type TokenUsageSnapshot struct {
	ThreadID, TurnID string
	LastRequest      *TokenCounts
	Turn             *TokenCounts
	Session          *TokenCounts
}

func (s TokenUsageSnapshot) Clone() TokenUsageSnapshot {
	clone := s
	for _, pair := range []struct {
		from *TokenCounts
		to   **TokenCounts
	}{{s.LastRequest, &clone.LastRequest}, {s.Turn, &clone.Turn}, {s.Session, &clone.Session}} {
		if pair.from != nil {
			c := *pair.from
			*pair.to = &c
		}
	}
	return clone
}
