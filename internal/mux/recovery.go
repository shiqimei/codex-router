package mux

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/shiqimei/codex-router/internal/backend"
	"github.com/shiqimei/codex-router/internal/protocol"
)

type recoveryPlan struct {
	message  protocol.Message
	excluded map[string]struct{}
}

func (m *Multiplexer) trackTurn(threadID string, message protocol.Message, owner string) {
	m.activityMu.Lock()
	defer m.activityMu.Unlock()
	m.recoveryPlans[threadID] = recoveryPlan{message: message, excluded: map[string]struct{}{owner: {}}}
}
func (m *Multiplexer) coolingDown(id string) bool {
	m.activityMu.Lock()
	defer m.activityMu.Unlock()
	return time.Now().Before(m.cooldowns[id])
}

// Model-side limits can arrive after turn/start succeeded, as a terminal event.
// Do not replay arbitrary failed tools or transient errors: only explicit quota errors.
func (m *Multiplexer) maybeRecoverQuota(in backend.Inbound) bool {
	if in.Message.Method != "turn/completed" {
		return false
	}
	var p struct {
		ThreadID string `json:"threadId"`
		Turn     struct {
			ID     string          `json:"id"`
			Status string          `json:"status"`
			Error  json.RawMessage `json:"error"`
		} `json:"turn"`
	}
	if json.Unmarshal(in.Message.Params, &p) != nil {
		return false
	}
	text := strings.ToLower(string(p.Turn.Error))
	quota := p.Turn.Status == "failed" && (strings.Contains(text, "usagelimitexceeded") || strings.Contains(text, "usage_limit") || strings.Contains(text, "rate_limit") || strings.Contains(text, "out of credits"))
	m.activityMu.Lock()
	plan, ok := m.recoveryPlans[p.ThreadID]
	if !quota {
		delete(m.recoveryPlans, p.ThreadID)
	} else {
		m.cooldowns[in.AccountID] = time.Now().Add(5 * time.Minute)
	}
	m.activityMu.Unlock()
	if !quota || !ok {
		return false
	}
	go func() {
		m.lifecycleMu.RLock()
		defer m.lifecycleMu.RUnlock()
		lock := m.threadLock(p.ThreadID)
		lock.Lock()
		defer lock.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 2*requestTimeout)
		defer cancel()
		plan.excluded[in.AccountID] = struct{}{}
		target, _, err := m.chooseAccountExcluding(ctx, plan.excluded)
		if err != nil {
			m.writeRaw(in.Raw)
			return
		}
		if _, err = m.switchLocked(ctx, SwitchInput{ThreadID: p.ThreadID, AccountID: target.ID}, false); err != nil {
			m.writeRaw(in.Raw)
			return
		}
		// Close the failed turn before the target emits a replacement started event.
		m.writeRaw(in.Raw)
		next := m.applyProvider(target.ID, plan.message)
		plan.excluded[target.ID] = struct{}{}
		m.activityMu.Lock()
		m.recoveryPlans[p.ThreadID] = plan
		m.activityMu.Unlock()
		child, _ := m.child(target.ID)
		response, err := child.Request(ctx, "turn/start", next.Params)
		if err != nil {
			m.publish(Event{Type: "recovery-failed", AccountID: target.ID, Message: err.Error(), Data: map[string]any{"threadId": p.ThreadID}})
			return
		}
		nextID := m.rememberTurn(p.ThreadID, response.Result)
		m.activityMu.Lock()
		m.turnAliases[p.ThreadID] = map[string]string{p.Turn.ID: nextID}
		m.activityMu.Unlock()
		m.publish(Event{Type: "thread-failed-over", AccountID: target.ID, Data: map[string]any{"threadId": p.ThreadID, "previousAccountId": in.AccountID, "turnId": nextID}})
	}()
	return true
}
