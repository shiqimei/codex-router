package mux

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shiqimei/codex-router/internal/state"
)

func (m *Multiplexer) ProviderDetails(id string) (state.ProviderDetails, error) {
	return m.store.ProviderDetails(id)
}
func (m *Multiplexer) providerIdle(ctx context.Context, id string) error {
	account, ok := m.store.Account(id)
	if !ok || account.Kind != "provider" || account.DeletedAt != 0 {
		return errors.New("provider not found")
	}
	child, ok := m.child(id)
	if !ok {
		return nil
	}
	for _, thread := range m.store.OwnedThreads(id) {
		if m.activeTurn(thread) != "" {
			return errors.New("stop the provider's running turns before editing or deleting it")
		}
	}
	cursor := ""
	for {
		params, _ := json.Marshal(map[string]any{"limit": 100, "cursor": func() any {
			if cursor == "" {
				return nil
			}
			return cursor
		}()})
		page, err := child.Request(ctx, "thread/loaded/list", params)
		if err != nil {
			return fmt.Errorf("cannot verify provider is idle: %w", err)
		}
		var loaded struct {
			Data       []string `json:"data"`
			NextCursor *string  `json:"nextCursor"`
		}
		if err = json.Unmarshal(page.Result, &loaded); err != nil {
			return err
		}
		for _, thread := range loaded.Data {
			params, _ := json.Marshal(map[string]any{"threadId": thread, "includeTurns": false})
			r, err := child.Request(ctx, "thread/read", params)
			if err != nil {
				if strings.Contains(err.Error(), "thread not loaded") {
					continue
				}
				return fmt.Errorf("cannot verify provider is idle: %w", err)
			}
			var result struct {
				Thread struct {
					Status struct {
						Type string `json:"type"`
					} `json:"status"`
					Turns []struct {
						Status string `json:"status"`
					} `json:"turns"`
				} `json:"thread"`
			}
			if err = json.Unmarshal(r.Result, &result); err != nil {
				return err
			}
			if result.Thread.Status.Type == "active" {
				return errors.New("stop the provider's running turns before editing or deleting it")
			}
			for _, t := range result.Thread.Turns {
				if t.Status == "inProgress" {
					return errors.New("stop the provider's running turns before editing or deleting it")
				}
			}
		}
		if loaded.NextCursor == nil || *loaded.NextCursor == "" {
			break
		}
		if *loaded.NextCursor == cursor {
			return errors.New("invalid loaded thread pagination")
		}
		cursor = *loaded.NextCursor
	}
	return nil
}
func (m *Multiplexer) restartProvider(ctx context.Context, account state.Account) error {
	for _, thread := range m.store.OwnedThreads(account.ID) {
		m.resumeNeeded.Store(thread, true)
	}
	m.childrenMu.Lock()
	old := m.children[account.ID]
	delete(m.children, account.ID)
	m.childrenMu.Unlock()
	if old != nil {
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := old.Stop(stopCtx)
		cancel()
		if err != nil {
			return err
		}
	}
	_, err := m.startChild(ctx, account)
	return err
}
func (m *Multiplexer) UpdateProvider(ctx context.Context, id string, input state.ProviderUpdate) (AccountSnapshot, error) {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	if err := m.providerIdle(ctx, id); err != nil {
		return AccountSnapshot{}, err
	}
	old, err := m.store.ProviderDetails(id)
	if err != nil {
		return AccountSnapshot{}, err
	}
	account, _ := m.store.Account(id)
	data, err := os.ReadFile(filepath.Join(account.CodexHome, "provider-env.json"))
	if err != nil {
		return AccountSnapshot{}, err
	}
	previousEnvironment := map[string]string{}
	if err = json.Unmarshal(data, &previousEnvironment); err != nil {
		return AccountSnapshot{}, err
	}
	updated, err := m.store.UpdateProvider(id, input)
	if err != nil {
		return AccountSnapshot{}, err
	}
	if err = m.restartProvider(ctx, updated); err != nil {
		current, readErr := m.store.ProviderDetails(id)
		if readErr != nil {
			return AccountSnapshot{}, errors.Join(err, readErr)
		}
		restored, restoreErr := m.store.UpdateProvider(id, state.ProviderUpdate{ProviderInput: state.ProviderInput{Label: old.Label, ConfigTOML: old.ConfigTOML, Environment: previousEnvironment, ModelsJSON: &old.ModelsJSON}, Revision: current.Revision})
		if restoreErr == nil {
			restoreCtx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			restoreErr = m.restartProvider(restoreCtx, restored)
			cancel()
		}
		return AccountSnapshot{}, errors.Join(fmt.Errorf("updated provider failed to start; attempted restoration of previous configuration: %w", err), restoreErr)
	}
	m.publishAccountRefresh(id)
	return m.accountSnapshot(ctx, id)
}
func (m *Multiplexer) DeleteProvider(ctx context.Context, id string) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	if err := m.providerIdle(ctx, id); err != nil {
		return err
	}
	if _, err := m.store.DeleteProvider(id); err != nil {
		return err
	}
	m.publish(Event{Type: "account-updated", AccountID: id})
	return nil
}
func (m *Multiplexer) RestoreProvider(ctx context.Context, id string) (AccountSnapshot, error) {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	a, err := m.store.RestoreProvider(id)
	if err != nil {
		return AccountSnapshot{}, err
	}
	if _, err = m.startChild(ctx, a); err != nil {
		return AccountSnapshot{}, err
	}
	m.publishAccountRefresh(id)
	return m.accountSnapshot(ctx, id)
}
