package mux

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shiqimei/codex-router/internal/backend"
	"github.com/shiqimei/codex-router/internal/protocol"
	"github.com/shiqimei/codex-router/internal/state"
)

type SwitchInput struct {
	ThreadID  string            `json:"threadId"`
	AccountID string            `json:"accountId"`
	Input     []json.RawMessage `json:"input,omitempty"`
}
type SwitchResult struct {
	ThreadID  string `json:"threadId"`
	AccountID string `json:"accountId"`
	TurnID    string `json:"turnId,omitempty"`
	Continued bool   `json:"continued"`
}

func (m *Multiplexer) threadLock(id string) *sync.Mutex {
	v, _ := m.threadLocks.LoadOrStore(id, &sync.Mutex{})
	return v.(*sync.Mutex)
}
func (m *Multiplexer) activeTurn(id string) string {
	m.activityMu.Lock()
	defer m.activityMu.Unlock()
	return m.activeTurns[id]
}

func (m *Multiplexer) applyProvider(accountID string, msg protocol.Message) protocol.Message {
	account, ok := m.store.Account(accountID)
	if !ok {
		return msg
	}
	var p map[string]any
	if json.Unmarshal(msg.Params, &p) != nil {
		return msg
	}
	model := m.providerModel(account, p, msg.Method)
	switch msg.Method {
	case "thread/start", "thread/resume", "thread/fork":
		if account.Kind == "provider" {
			p["modelProvider"] = account.Provider
			p["model"] = model
			if mode, ok := p["collaborationMode"].(map[string]any); ok {
				if settings, ok := mode["settings"].(map[string]any); ok {
					settings["model"] = model
				}
			}
		} else {
			p["modelProvider"] = "openai"
		}
	case "turn/start", "thread/settings/update":
		if account.Kind == "provider" {
			p["model"] = model
			if mode, ok := p["collaborationMode"].(map[string]any); ok {
				if settings, ok := mode["settings"].(map[string]any); ok {
					settings["model"] = model
				}
			}
		}
	}
	if account.Kind == "provider" {
		switch msg.Method {
		case "thread/start", "thread/resume", "thread/fork":
			m.normalizeProviderEffort(account, model, p, "reasoningEffort")
		case "turn/start", "thread/settings/update":
			m.normalizeProviderEffort(account, model, p, "effort")
		}
	}
	msg.Params, _ = json.Marshal(p)
	return msg
}

// Thread mutations and migrations share a lock. Inbound notifications never take
// this lock, so interrupt -> completion can drain while the handoff is pending.
func (m *Multiplexer) routeThreadRequest(message protocol.Message, threadID, owner string) {
	m.lifecycleMu.RLock()
	defer m.lifecycleMu.RUnlock()
	lock := m.threadLock(threadID)
	lock.Lock()
	defer lock.Unlock()
	if id, ok := m.store.ThreadOwner(threadID); ok {
		owner = id
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*requestTimeout)
	defer cancel()
	if a, ok := m.store.Account(owner); ok && a.DeletedAt != 0 && strings.HasPrefix(message.Method, "turn/") {
		fallback, _, err := m.chooseAccount(ctx)
		if err != nil {
			m.write(protocol.Failure(message.ID, -32023, "select an available connection to continue this deleted provider's thread"))
			return
		}
		if _, err = m.switchLocked(ctx, SwitchInput{ThreadID: threadID, AccountID: fallback.ID}, false); err != nil {
			m.write(protocol.Failure(message.ID, -32023, err.Error()))
			return
		}
		owner = fallback.ID
	}
	if _, needed := m.resumeNeeded.Load(threadID); needed && strings.HasPrefix(message.Method, "turn/") {
		child, ok := m.child(owner)
		if !ok {
			m.write(protocol.Failure(message.ID, -32023, "provider unavailable"))
			return
		}
		params, _ := json.Marshal(map[string]any{"threadId": threadID})
		req := m.applyProvider(owner, protocol.Request("thread/resume", nil, params))
		r, err := child.Request(ctx, req.Method, req.Params)
		if err != nil {
			m.write(protocol.Failure(message.ID, -32023, err.Error()))
			return
		}
		m.resumeNeeded.Delete(threadID)
		m.publishThreadSettings(threadID, r.Result)
	}
	if message.Method == "turn/steer" {
		var p map[string]any
		_ = json.Unmarshal(message.Params, &p)
		old, _ := p["expectedTurnId"].(string)
		m.activityMu.Lock()
		replacement := m.turnAliases[threadID][old]
		m.activityMu.Unlock()
		if replacement != "" {
			p["expectedTurnId"] = replacement
			message.Params, _ = json.Marshal(p)
		}
	}
	if message.Method == "turn/start" {
		m.trackTurn(threadID, message, owner)
	}
	message = m.applyProvider(owner, message)
	child, ok := m.child(owner)
	if !ok {
		m.write(protocol.Failure(message.ID, -32023, "account unavailable"))
		return
	}
	response, err := child.Request(ctx, message.Method, message.Params)
	if err != nil && message.Method == "turn/start" && isUsageLimitResponse(response) {
		excluded := map[string]struct{}{owner: {}}
		for len(excluded) < len(m.store.Accounts()) {
			fallback, _, chooseErr := m.chooseAccountExcluding(ctx, excluded)
			if chooseErr != nil {
				break
			}
			if _, switchErr := m.switchLocked(ctx, SwitchInput{ThreadID: threadID, AccountID: fallback.ID}, false); switchErr != nil {
				m.write(protocol.Failure(message.ID, -32027, switchErr.Error()))
				return
			}
			owner = fallback.ID
			excluded[owner] = struct{}{}
			child, _ = m.child(owner)
			message = m.applyProvider(owner, message)
			response, err = child.Request(ctx, message.Method, message.Params)
			if err == nil || !isUsageLimitResponse(response) {
				break
			}
		}
	}
	if err != nil {
		if response.Error != nil {
			response.ID = message.ID
			m.write(response)
		} else {
			m.write(protocol.Failure(message.ID, -32023, err.Error()))
		}
		return
	}
	response.ID = message.ID
	if message.Method == "turn/start" || message.Method == "thread/settings/update" {
		m.rememberRequestedModel(threadID, owner, message.Params)
	}
	m.learnThreadOwner(externalRoute{method: message.Method}, owner, response.Result)
	if message.Method == "turn/start" {
		m.rememberTurn(threadID, response.Result)
	}
	m.write(response)
}

func (m *Multiplexer) rememberTurn(threadID string, result json.RawMessage) string {
	var v struct {
		Turn struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"turn"`
	}
	_ = json.Unmarshal(result, &v)

	return v.Turn.ID
}
func (m *Multiplexer) observeThreadEvent(in backend.Inbound) bool {
	if in.Message.Method == "" {
		return true
	}
	var p struct {
		ThreadID string `json:"threadId"`
		Thread   struct {
			ID string `json:"id"`
		} `json:"thread"`
		Turn struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	_ = json.Unmarshal(in.Message.Params, &p)
	id := p.ThreadID
	if id == "" {
		id = p.Thread.ID
	}
	if id == "" {
		return true
	}
	if owner, ok := m.store.ThreadOwner(id); ok && owner != in.AccountID {
		return false
	}
	m.activityMu.Lock()
	defer m.activityMu.Unlock()
	switch in.Message.Method {
	case "turn/started":
		m.activeTurns[id] = p.Turn.ID
	case "turn/completed":
		if m.activeTurns[id] == p.Turn.ID {
			delete(m.activeTurns, id)
			if !m.migrating[id] {
				delete(m.turnAliases, id)
			}
		}
	}
	return true
}

func (m *Multiplexer) SwitchThread(ctx context.Context, input SwitchInput) (SwitchResult, error) {
	m.lifecycleMu.RLock()
	defer m.lifecycleMu.RUnlock()
	if input.ThreadID == "" || input.AccountID == "" {
		return SwitchResult{}, errors.New("threadId and accountId are required")
	}
	lock := m.threadLock(input.ThreadID)
	lock.Lock()
	defer lock.Unlock()
	return m.switchLocked(ctx, input, true)
}
func (m *Multiplexer) switchLocked(ctx context.Context, input SwitchInput, continueActive bool) (SwitchResult, error) {
	result := SwitchResult{ThreadID: input.ThreadID, AccountID: input.AccountID}
	targetAccount, ok := m.store.Account(input.AccountID)
	if !ok || !targetAccount.Enabled {
		return result, errors.New("target account unavailable")
	}
	owner, ok := m.store.ThreadOwner(input.ThreadID)
	if !ok {
		if c, exists := m.store.Controller(); exists {
			owner = c.ID
		} else {
			return result, errors.New("thread owner unknown")
		}
	}
	if owner == input.AccountID {
		return result, nil
	}
	m.activityMu.Lock()
	m.migrating[input.ThreadID] = true
	m.activityMu.Unlock()
	defer func() { m.activityMu.Lock(); delete(m.migrating, input.ThreadID); m.activityMu.Unlock() }()
	source, ok := m.child(owner)
	if !ok {
		return result, errors.New("source account unavailable")
	}
	target, ok := m.child(input.AccountID)
	if !ok {
		return result, errors.New("target account unavailable")
	}
	// Read before interrupt, so a bad thread cannot affect any other work.
	readParams, _ := json.Marshal(map[string]any{"threadId": input.ThreadID, "includeTurns": true})
	read, err := source.Request(ctx, "thread/read", readParams)
	if err != nil {
		return result, err
	}
	var saved struct {
		Thread struct {
			Path  string `json:"path"`
			CWD   string `json:"cwd"`
			Turns []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"turns"`
		} `json:"thread"`
	}
	if err = json.Unmarshal(read.Result, &saved); err != nil {
		return result, err
	}
	if saved.Thread.Path == "" {
		return result, errors.New("thread has no persisted rollout; ephemeral threads cannot migrate")
	}
	active := m.activeTurn(input.ThreadID)
	if active == "" {
		for _, t := range saved.Thread.Turns {
			if t.Status == "inProgress" {
				active = t.ID
			}
		}
	}
	if active != "" {
		m.activityMu.Lock()
		m.activeTurns[input.ThreadID] = active
		m.activityMu.Unlock()
		params, _ := json.Marshal(map[string]any{"threadId": input.ThreadID, "turnId": active})
		if _, err = source.Request(ctx, "turn/interrupt", params); err != nil {
			return result, fmt.Errorf("interrupt source: %w", err)
		}
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for m.activeTurn(input.ThreadID) != "" {
			select {
			case <-ctx.Done():
				return result, fmt.Errorf("wait for source to stop: %w", ctx.Err())
			case <-ticker.C:
			}
		}
	}
	committed := false
	defer func() {
		if !committed {
			rollbackParams, _ := json.Marshal(map[string]any{"threadId": input.ThreadID, "path": saved.Thread.Path, "cwd": saved.Thread.CWD})
			rollback := m.applyProvider(owner, protocol.Request("thread/resume", nil, rollbackParams))
			restoreCtx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			defer cancel()
			_, _ = source.Request(restoreCtx, "thread/resume", rollback.Params)
		}
	}()
	// Release the original rollout writer before opening it in another process.
	unload, _ := json.Marshal(map[string]any{"threadId": input.ThreadID})
	if _, err = source.Request(ctx, "thread/unsubscribe", unload); err != nil {
		return result, fmt.Errorf("release source thread: %w", err)
	}
	// Current Codex indexes rollouts within CODEX_HOME even when path is supplied.
	// Copy the fully drained history into the target home, keeping its thread UUID.
	targetPath := filepath.Join(targetAccount.CodexHome, "sessions", rolloutDate(saved.Thread.Path), filepath.Base(saved.Thread.Path))
	data, copyErr := os.ReadFile(saved.Thread.Path)
	if copyErr != nil {
		return result, copyErr
	}
	data, copyErr = portableHistory(data)
	if copyErr != nil {
		return result, copyErr
	}
	if err = os.MkdirAll(filepath.Dir(targetPath), 0700); err != nil {
		return result, err
	}
	if err = os.WriteFile(targetPath+".handoff", data, 0600); err != nil {
		return result, err
	}
	if err = os.Rename(targetPath+".handoff", targetPath); err != nil {
		return result, err
	}
	params, _ := json.Marshal(map[string]any{"threadId": input.ThreadID, "path": targetPath, "cwd": saved.Thread.CWD})
	if targetAccount.Kind != "provider" {
		var values map[string]any
		_ = json.Unmarshal(params, &values)
		if model := m.store.DefaultModel(targetAccount); model != "" {
			values["model"] = model
		}
		params, _ = json.Marshal(values)
	}
	request := m.applyProvider(input.AccountID, protocol.Request("thread/resume", nil, params))
	resumed, err := target.Request(ctx, "thread/resume", request.Params)
	if err != nil {
		rollbackParams, _ := json.Marshal(map[string]any{"threadId": input.ThreadID, "path": saved.Thread.Path, "cwd": saved.Thread.CWD})
		rollback := m.applyProvider(owner, protocol.Request("thread/resume", nil, rollbackParams))
		_, restoreErr := source.Request(ctx, "thread/resume", rollback.Params)
		return result, fmt.Errorf("target resume failed: %w (source restore: %v)", err, restoreErr)
	}
	if got := threadIDFromResult(resumed.Result); got != input.ThreadID {
		return result, errors.New("backend changed thread identity during migration")
	}
	if err = m.store.SetThreadOwner(input.ThreadID, input.AccountID); err != nil {
		return result, err
	}
	committed = true
	m.publishThreadSettings(input.ThreadID, resumed.Result)
	m.publish(Event{Type: "thread-switched", AccountID: input.AccountID, Data: result})
	if (active != "" && continueActive) || len(input.Input) > 0 {
		inputs := input.Input
		if len(inputs) == 0 {
			inputs = []json.RawMessage{json.RawMessage(`{"type":"text","text":"Continue the interrupted task from the saved conversation. Preserve completed work and follow the latest user instructions.","text_elements":[]}`)}
		}
		p, _ := json.Marshal(map[string]any{"threadId": input.ThreadID, "input": inputs})
		req := m.applyProvider(input.AccountID, protocol.Request("turn/start", nil, p))
		m.trackTurn(input.ThreadID, req, input.AccountID)
		response, startErr := target.Request(ctx, "turn/start", req.Params)
		if startErr != nil {
			return result, fmt.Errorf("thread moved; continuation failed: %w", startErr)
		}
		result.TurnID = m.rememberTurn(input.ThreadID, response.Result)
		result.Continued = true
		if active != "" {
			m.activityMu.Lock()
			if m.turnAliases[input.ThreadID] == nil {
				m.turnAliases[input.ThreadID] = map[string]string{}
			}
			for old := range m.turnAliases[input.ThreadID] {
				m.turnAliases[input.ThreadID][old] = result.TurnID
			}
			m.turnAliases[input.ThreadID][active] = result.TurnID
			m.activityMu.Unlock()
		}
	}
	return result, nil
}
func (m *Multiplexer) AddProvider(ctx context.Context, input state.ProviderInput) (AccountSnapshot, error) {
	a, err := m.store.AddProvider(input)
	if err != nil {
		return AccountSnapshot{}, err
	}
	if _, err = m.startChild(ctx, a); err != nil {
		disabled := false
		_, _ = m.store.UpdateAccount(a.ID, nil, &disabled)
		return AccountSnapshot{}, err
	}
	m.publishAccountRefresh(a.ID)
	return m.accountSnapshot(ctx, a.ID)
}

func (m *Multiplexer) DefaultAccount() string { return m.store.DefaultAccount() }
func (m *Multiplexer) SetDefaultAccount(id string) error {
	if err := m.store.SetDefaultAccount(id); err != nil {
		return err
	}
	m.publish(Event{Type: "routing-updated", AccountID: id})
	return nil
}

func rolloutDate(path string) string {
	base := filepath.Base(path)
	if len(base) >= 18 {
		if date, err := time.Parse("2006-01-02", base[8:18]); err == nil {
			return date.Format("2006/01/02")
		}
	}
	return time.Now().Format("2006/01/02")
}

// Resume returns authoritative settings but does not notify other clients when
// the selected model is already effective. Publish them for the desktop cache.
func (m *Multiplexer) publishThreadSettings(threadID string, result json.RawMessage) {
	var effective map[string]any
	if json.Unmarshal(result, &effective) != nil {
		return
	}
	if model, ok := effective["model"].(string); ok {
		if owner, ok := m.store.ThreadOwner(threadID); ok {
			_ = m.store.SetThreadModel(threadID, owner, model)
		}
	}
	if effective["collaborationMode"] == nil {
		return
	}
	settings := map[string]any{}
	for _, key := range []string{"cwd", "approvalPolicy", "approvalsReviewer", "activePermissionProfile", "model", "modelProvider", "serviceTier", "collaborationMode", "multiAgentMode", "disabledPluginIds"} {
		settings[key] = effective[key]
	}
	settings["sandboxPolicy"] = effective["sandbox"]
	settings["effort"] = effective["reasoningEffort"]
	settings["summary"] = nil
	settings["personality"] = nil
	params, _ := json.Marshal(map[string]any{"threadId": threadID, "threadSettings": settings})
	m.write(protocol.Message{Method: "thread/settings/updated", Params: params})
}
