package main

import "sync"

// The main WebView owns Run(). Overlay WebViews have unpumped Dispatch queues,
// so the caller supplies the main dispatcher and the overlay's evaluator.
type skillOverlayPusher struct {
	mu       sync.Mutex
	latest   string
	pending  bool
	closed   bool
	dispatch func(func())
	evaluate func(string)
}

var skillOverlayPushTarget struct {
	sync.RWMutex
	pusher *skillOverlayPusher
}

func startNativeSkillOverlayPush(dispatch func(func()), evaluate func(string)) func() {
	p := &skillOverlayPusher{dispatch: dispatch, evaluate: evaluate}
	// Register and snapshot atomically with state composition, so a new live
	// frame cannot be overwritten by the older startup snapshot.
	skillOverlayState.RLock()
	skillOverlayPushTarget.Lock()
	skillOverlayPushTarget.pusher = p
	skillOverlayPushTarget.Unlock()
	p.queue(skillOverlayState.data)
	skillOverlayState.RUnlock()
	return func() {
		skillOverlayPushTarget.Lock()
		if skillOverlayPushTarget.pusher == p {
			skillOverlayPushTarget.pusher = nil
		}
		skillOverlayPushTarget.Unlock()
		p.mu.Lock()
		p.closed, p.latest = true, ""
		p.mu.Unlock()
	}
}

func pushNativeSkillOverlayState(data []byte) {
	skillOverlayPushTarget.RLock()
	p := skillOverlayPushTarget.pusher
	skillOverlayPushTarget.RUnlock()
	if p != nil {
		p.queue(data)
	}
}

func (p *skillOverlayPusher) queue(data []byte) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.latest = string(data)
	if p.pending {
		p.mu.Unlock()
		return
	}
	p.pending = true
	p.mu.Unlock()
	p.dispatch(func() {
		p.mu.Lock()
		data, closed := p.latest, p.closed
		p.pending = false
		p.mu.Unlock()
		if !closed && data != "" {
			// JSON encoding escapes text; never interpolate names as JS strings.
			p.evaluate(`window.dispatchEvent(new CustomEvent("dilmeter-skill-overlay-state",{detail:` + data + `}));`)
		}
	})
}
