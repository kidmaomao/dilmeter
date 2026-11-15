package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSkillOverlayPushCoalescesWithoutWaitingForPoll(t *testing.T) {
	preserveBurstPreviewState(t)
	var callbacks []func()
	var scripts []string
	stop := startNativeSkillOverlayPush(func(callback func()) {
		callbacks = append(callbacks, callback)
	}, func(script string) { scripts = append(scripts, script) })
	defer stop()
	const now int64 = 100000
	live := nativeSkillOverlayMessage{Mechanics: []nativeBossMechanicOverlayItem{{
		Key: perlonTornadoKey, Name: `";window.bad=true;//<script>`, StartedAtMs: now, EndsAtMs: now + 3000,
	}}}
	raw, _ := json.Marshal(live)
	setSkillOverlayStateAt(raw, now)
	first := readBurstPreviewState(t)
	// An unrelated preview and a live frame in the same millisecond cannot
	// replace each other, or create an unbounded UI dispatch queue.
	setReminderPreview(reminderPreviewRequest{DurationMs: 3600, Items: []nativeSkillOverlayItem{{SkillID: 59000, ReadyAtMs: now + 900}}}, now)
	setSkillOverlayStateAt(raw, now)
	latest := readBurstPreviewState(t)
	if latest.Revision <= first.Revision || len(callbacks) != 1 || len(scripts) != 0 {
		t.Fatalf("state did not immediately queue one latest frame: callbacks=%d, revisions=%d/%d", len(callbacks), first.Revision, latest.Revision)
	}
	callbacks[0]() // The main UI dispatcher; no poll or heartbeat is required.
	if len(scripts) != 1 {
		t.Fatal("native push did not evaluate on UI dispatch")
	}
	const prefix = `window.dispatchEvent(new CustomEvent("dilmeter-skill-overlay-state",{detail:`
	const suffix = `}));`
	if !strings.HasPrefix(scripts[0], prefix) || !strings.HasSuffix(scripts[0], suffix) || strings.Contains(scripts[0], "<script>") {
		t.Fatalf("unsafe or malformed native push: %s", scripts[0])
	}
	var pushed nativeSkillOverlayMessage
	if err := json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimPrefix(scripts[0], prefix), suffix)), &pushed); err != nil {
		t.Fatal(err)
	}
	if pushed.Revision != latest.Revision || len(pushed.Mechanics) != 1 || len(pushed.Items) != 1 || pushed.Mechanics[0].Name != live.Mechanics[0].Name {
		t.Fatalf("push lost the latest composed state: %+v", pushed)
	}
	// A later frame must schedule another callback; shutdown must make an
	// already queued callback harmless before destroying the overlay WebView.
	setSkillOverlayStateAt(raw, now+1)
	if len(callbacks) != 2 {
		t.Fatal("new state never dispatched after first delivery")
	}
	stop()
	callbacks[1]()
	setSkillOverlayStateAt(raw, now+2)
	if len(scripts) != 1 || len(callbacks) != 2 {
		t.Fatal("closed overlay was evaluated or dispatched")
	}
}

func TestSkillOverlayPushCanQueueDuringEvaluation(t *testing.T) {
	var callbacks []func()
	var delivered []string
	p := &skillOverlayPusher{dispatch: func(callback func()) { callbacks = append(callbacks, callback) }}
	p.evaluate = func(script string) {
		delivered = append(delivered, script)
		if len(delivered) == 1 {
			p.queue([]byte(`{"revision":2}`))
		}
	}
	p.queue([]byte(`{"revision":1}`))
	callbacks[0]()
	if len(callbacks) != 2 {
		t.Fatal("state arriving during evaluation was lost")
	}
	callbacks[1]()
	if len(delivered) != 2 || !strings.Contains(delivered[1], `"revision":2`) {
		t.Fatal("newest state was not delivered")
	}
}
