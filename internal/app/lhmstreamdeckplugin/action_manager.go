package lhmstreamdeckplugin

import (
	"fmt"
	"sync"
	"time"
)

type actionManager struct {
	mux     sync.RWMutex
	actions map[string]*actionData
	lastRun map[string]time.Time
}

func newActionManager() *actionManager {
	return &actionManager{
		actions: make(map[string]*actionData),
		lastRun: make(map[string]time.Time),
	}
}

func (tm *actionManager) Run(updateTiles func(*actionData)) {
	go func() {
		ticker := time.NewTicker(time.Second)
		for range ticker.C {
			now := time.Now()
			var toUpdate []*actionData
			tm.mux.Lock()
			for _, data := range tm.actions {
				if data.settings.IsValid {
					last := tm.lastRun[data.context]
					if data.settings.InErrorState && now.Sub(last) < 5*time.Second {
						continue
					}
					toUpdate = append(toUpdate, data)
				}
			}
			tm.mux.Unlock()

			for _, data := range toUpdate {
				updateTiles(data)
				tm.mux.Lock()
				tm.lastRun[data.context] = now
				tm.mux.Unlock()
			}
		}
	}()
}

func (tm *actionManager) SetAction(action, context string, settings *actionSettings) {
	tm.mux.Lock()
	tm.actions[context] = &actionData{action, context, settings}
	tm.mux.Unlock()
}

func (tm *actionManager) RemoveAction(context string) {
	tm.mux.Lock()
	delete(tm.actions, context)
	delete(tm.lastRun, context)
	tm.mux.Unlock()
}

func (tm *actionManager) getSettings(context string) (actionSettings, error) {
	tm.mux.RLock()
	data, ok := tm.actions[context]
	tm.mux.RUnlock()
	if !ok {
		return actionSettings{}, fmt.Errorf("getSettings invalid key: %s", context)
	}
	// return full copy of settings, not reference to stored settings
	return *data.settings, nil
}
