package lhmstreamdeckplugin

import (
	"math"
	"time"
)

type thresholdRuntimeState struct {
	PendingSince         time.Time
	CooldownUntil        time.Time
	Active               bool
	Latched              bool
	SuppressedUntilClear bool
	SnapshotPending      bool
	LatchedValue         float64
	LatchedGraphValue    float64
	LatchedDisplayText   string
	LatchedAlertText     string
}

func resetThresholdSnapshot(state *thresholdRuntimeState) {
	if state == nil {
		return
	}
	state.SnapshotPending = false
	state.LatchedValue = 0
	state.LatchedGraphValue = 0
	state.LatchedDisplayText = ""
	state.LatchedAlertText = ""
}

func clearStickyThresholdState(state *thresholdRuntimeState) bool {
	if state == nil || !state.Latched {
		return false
	}
	state.PendingSince = time.Time{}
	state.CooldownUntil = time.Time{}
	state.Active = false
	state.Latched = false
	state.SuppressedUntilClear = true
	resetThresholdSnapshot(state)
	return true
}

func (p *Plugin) markThresholdDirty(context string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.thresholdDirty[context] = true
}

func (p *Plugin) consumeThresholdDirty(context string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	dirty := p.thresholdDirty[context]
	delete(p.thresholdDirty, context)
	return dirty
}

func (p *Plugin) clearThresholdRuntimeState(context string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.thresholdStates, context)
	delete(p.thresholdDirty, context)
}

func (p *Plugin) resetThresholdRuntimeState(context, thresholdID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if thresholdID == "" {
		delete(p.thresholdStates, context)
	} else if states, ok := p.thresholdStates[context]; ok {
		delete(states, thresholdID)
		if len(states) == 0 {
			delete(p.thresholdStates, context)
		}
	}
	p.thresholdDirty[context] = true
}

func (p *Plugin) clearStickyThreshold(context, thresholdID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	states := p.thresholdStates[context]
	if states == nil {
		return false
	}

	if thresholdID != "" {
		state := states[thresholdID]
		if clearStickyThresholdState(state) {
			p.thresholdDirty[context] = true
			return true
		}
		return false
	}

	for _, state := range states {
		if clearStickyThresholdState(state) {
			p.thresholdDirty[context] = true
			return true
		}
	}

	return false
}

func stickySnapshotShouldUpdate(t *Threshold, currentValue, snapshotValue float64) bool {
	if t == nil {
		return false
	}

	switch t.Operator {
	case ">", ">=":
		return currentValue > snapshotValue
	case "<", "<=":
		return currentValue < snapshotValue
	case "==":
		return math.Abs(currentValue-t.Value) > math.Abs(snapshotValue-t.Value)
	default:
		return false
	}
}

func (p *Plugin) resolveThresholdDisplay(
	context string,
	t *Threshold,
	liveValue float64,
	liveGraphValue float64,
	liveDisplayText string,
	liveAlertText string,
) (string, string, float64, bool) {
	if t == nil || !t.Sticky {
		return liveDisplayText, liveAlertText, liveGraphValue, false
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	states := p.thresholdStates[context]
	if states == nil {
		return liveDisplayText, liveAlertText, liveGraphValue, false
	}
	state := states[t.ID]
	if state == nil || !state.Latched {
		return liveDisplayText, liveAlertText, liveGraphValue, false
	}

	if state.SnapshotPending || state.LatchedDisplayText == "" || stickySnapshotShouldUpdate(t, liveValue, state.LatchedValue) {
		state.LatchedValue = liveValue
		state.LatchedGraphValue = liveGraphValue
		state.LatchedDisplayText = liveDisplayText
		state.LatchedAlertText = liveAlertText
		state.SnapshotPending = false
		return state.LatchedDisplayText, state.LatchedAlertText, state.LatchedGraphValue, false
	}

	return state.LatchedDisplayText, state.LatchedAlertText, state.LatchedGraphValue, true
}

func thresholdDurationMs(ms int) time.Duration {
	if ms <= 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}

func thresholdClearConditionMet(value float64, t *Threshold) bool {
	h := t.Hysteresis
	if h < 0 {
		h = 0
	}

	switch t.Operator {
	case ">":
		return value <= t.Value-h
	case ">=":
		return value < t.Value-h
	case "<":
		return value >= t.Value+h
	case "<=":
		return value > t.Value+h
	case "==":
		if h <= 0 {
			return !evaluateThreshold(value, t.Value, t.Operator)
		}
		return math.Abs(value-t.Value) > h
	default:
		return true
	}
}

func evaluateThresholdState(value float64, t *Threshold, state *thresholdRuntimeState, now time.Time) bool {
	if state == nil {
		return false
	}
	if !t.Enabled || t.Operator == "" {
		*state = thresholdRuntimeState{}
		return false
	}

	if state.SuppressedUntilClear {
		if thresholdClearConditionMet(value, t) {
			state.SuppressedUntilClear = false
		} else {
			state.PendingSince = time.Time{}
			return false
		}
	}

	rawMatch := evaluateThreshold(value, t.Value, t.Operator)

	if state.Latched {
		return true
	}

	if state.Active {
		if thresholdClearConditionMet(value, t) {
			state.PendingSince = time.Time{}
			state.Active = false
			state.Latched = false
			resetThresholdSnapshot(state)
			if cooldown := thresholdDurationMs(t.CooldownMs); cooldown > 0 {
				state.CooldownUntil = now.Add(cooldown)
			} else {
				state.CooldownUntil = time.Time{}
			}
			return false
		}
		return true
	}

	if !state.CooldownUntil.IsZero() {
		if now.Before(state.CooldownUntil) {
			state.PendingSince = time.Time{}
			return false
		}
		state.CooldownUntil = time.Time{}
	}

	if !rawMatch {
		state.PendingSince = time.Time{}
		return false
	}

	dwell := thresholdDurationMs(t.DwellMs)
	if dwell > 0 {
		if state.PendingSince.IsZero() {
			state.PendingSince = now
			return false
		}
		if now.Sub(state.PendingSince) < dwell {
			return false
		}
	}

	state.PendingSince = time.Time{}
	state.Active = true
	if t.Sticky {
		state.Latched = true
		state.SnapshotPending = true
		state.LatchedValue = 0
		state.LatchedGraphValue = 0
		state.LatchedDisplayText = ""
		state.LatchedAlertText = ""
	}
	return true
}

func (p *Plugin) evaluateThresholds(context string, value float64, thresholds []Threshold, now time.Time) *Threshold {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(thresholds) == 0 {
		delete(p.thresholdStates, context)
		return nil
	}

	states := p.thresholdStates[context]
	if states == nil {
		states = make(map[string]*thresholdRuntimeState)
		p.thresholdStates[context] = states
	}

	seen := make(map[string]struct{}, len(thresholds))
	var active *Threshold
	for i := range thresholds {
		t := &thresholds[i]
		seen[t.ID] = struct{}{}
		state := states[t.ID]
		if state == nil {
			state = &thresholdRuntimeState{}
			states[t.ID] = state
		}
		if evaluateThresholdState(value, t, state, now) {
			active = t
		}
	}

	for id := range states {
		if _, ok := seen[id]; !ok {
			delete(states, id)
		}
	}
	if len(states) == 0 {
		delete(p.thresholdStates, context)
	}

	return active
}
