package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/util"
)

func TestObsClockProbeMatchesLiveEventClock(t *testing.T) {
	now := time.Now()
	p := newSkillTestPublisher()
	p.serverScope, p.obsObservedAt, p.lastSentEventAt = "198.51.100.1", now, time.Now()
	e := &event.EventSkillAction{EventBase: event.EventBase{EventId: 10, At: now.Add(time.Hour).Unix(), Id: "100"}, AtMs: now.Add(time.Hour).UnixMilli(), SkillId: 59085}
	p.publish(e)
	if e.ObsClock == nil || e.ObsClock.Session != util.ObsClockSession || e.ObsClock.AtMs != util.ObsClockAt(now) || e.AtMs != now.Add(time.Hour).UnixMilli() {
		t.Fatalf("wrong clock stamp or changed capture time: %+v", e)
	}
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8030/api/obs_clock?nonce=test-clock-nonce-1234", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	handleObsClock(w, req)
	var got struct {
		Version int
		Session string
		NowMs   int64
		Nonce   string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || got.Version != 1 || got.Session != e.ObsClock.Session || got.NowMs < e.ObsClock.AtMs || got.Nonce != "test-clock-nonce-1234" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("invalid clock response: %+v %s", got, w.Body.String())
	}
	// Sending a pending batch must not freshen its observation stamp.
	stamp := *e.ObsClock
	p.publishNow()
	if *e.ObsClock != stamp {
		t.Fatal("send retimestamped an old event")
	}
}

func TestObsClockOfflineAndSnapshotDoNotBecomeLive(t *testing.T) {
	for _, scenario := range []struct {
		scope string
		at    time.Time
	}{{"", time.Now()}, {"198.51.100.1", time.Time{}}} {
		if obsEventClock(event.EventIdSkillAction, scenario.scope, scenario.at) != nil {
			t.Fatal("offline event acquired live clock")
		}
	}
	if obsEventClock(event.EventIdDamage, "198.51.100.1", time.Now()) != nil {
		t.Fatal("unneeded damage clock metadata")
	}
}

func TestObsClockEndpointRejectsRemoteMutationAndBadNonce(t *testing.T) {
	for _, scenario := range []struct {
		method, remote, nonce string
		code                  int
	}{{"GET", "192.0.2.1:1234", "test-clock-nonce-1234", 403}, {"POST", "127.0.0.1:1234", "test-clock-nonce-1234", 405}, {"GET", "127.0.0.1:1234", "", 400}} {
		r := httptest.NewRequest(scenario.method, "http://127.0.0.1:8030/api/obs_clock?nonce="+scenario.nonce, nil)
		r.RemoteAddr = scenario.remote
		w := httptest.NewRecorder()
		handleObsClock(w, r)
		if w.Code != scenario.code {
			t.Fatalf("got %d want %d", w.Code, scenario.code)
		}
	}
}
