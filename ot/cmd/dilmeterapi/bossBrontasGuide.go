package main

// This is an observation consumer, not a guessed packet decoder. Brontas ray
// opcodes are not established by the captures checked into this repository.
import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const brontasGuideFreshMs int64 = 500
const brontasGuideReactionMs int64 = 300
const brontasGuideGuardMs int64 = 150

type brontasPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Coordinates are relative to the selected mineral: radius=1, +X east, +Y
// north, before display rotation. Geometry must include ALL observed rays,
// including those from the opposite player and rays of unknown origin.
type brontasGuidePlayer struct {
	brontasPoint
	Radius float64 `json:"radius"`
	Speed  float64 `json:"speed"` // mineral radii / second
}

type brontasGuideRay struct {
	ID          string   `json:"id"`
	Axis        string   `json:"axis"` // horizontal or vertical, in map space
	WarningAtMs int64    `json:"warningAtMs"`
	LethalAtMs  int64    `json:"lethalAtMs"`
	EndsAtMs    int64    `json:"endsAtMs"`
	Offset      *float64 `json:"offset,omitempty"`
	HalfWidth   *float64 `json:"halfWidth,omitempty"`
}

// A complete snapshot, sent at least every 200ms by the existing recognizer.
// Explicit, absolute deadlines prevent network delay from restarting a timer.
type brontasGuideFrame struct {
	Session          string              `json:"session"`
	Sequence         uint64              `json:"sequence"`
	BossID           string              `json:"bossId"`
	ObservedAtMs     int64               `json:"observedAtMs"`
	Active           bool                `json:"active"`
	Clock            int                 `json:"clock"`
	GeometryComplete bool                `json:"geometryComplete"`
	Player           *brontasGuidePlayer `json:"player,omitempty"`
	Rays             []brontasGuideRay   `json:"rays"`
}

type nativeBrontasGuideView struct {
	Status       string              `json:"status"`
	Instruction  string              `json:"instruction"`
	Detail       string              `json:"detail"`
	Clock        int                 `json:"clock"`
	ValidUntilMs int64               `json:"validUntilMs"`
	LethalAtMs   int64               `json:"lethalAtMs,omitempty"`
	Player       *brontasGuidePlayer `json:"player,omitempty"`
	Target       *brontasPoint       `json:"target,omitempty"`
	Rays         []brontasGuideRay   `json:"rays,omitempty"`
}

func validBrontasClock(clock int) bool {
	return clock == 12 || clock == 3 || clock == 6 || clock == 9
}

func finiteBrontasNumber(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }

func validateBrontasGuideFrame(f brontasGuideFrame, nowMs int64) error {
	if len(f.Session) == 0 || len(f.Session) > 64 || f.Sequence == 0 || len(f.BossID) == 0 || len(f.BossID) > 32 ||
		f.ObservedAtMs > nowMs+50 || f.ObservedAtMs < nowMs-brontasGuideFreshMs || len(f.Rays) > 32 {
		return errors.New("invalid or stale Brontas observation")
	}
	if !f.Active {
		return nil
	}
	if f.Clock != 0 && !validBrontasClock(f.Clock) {
		return errors.New("invalid mineral clock")
	}
	if f.Player != nil {
		p := f.Player
		if !finiteBrontasNumber(p.X) || !finiteBrontasNumber(p.Y) || math.Abs(p.X) > 20 || math.Abs(p.Y) > 20 ||
			!finiteBrontasNumber(p.Radius) || p.Radius <= 0 || p.Radius > .4 ||
			!finiteBrontasNumber(p.Speed) || p.Speed <= 0 || p.Speed > 20 {
			return errors.New("invalid normalized player geometry")
		}
	}
	if f.GeometryComplete && (f.Player == nil || !validBrontasClock(f.Clock)) {
		return errors.New("complete geometry requires player and mineral")
	}
	ids := make(map[string]bool)
	for _, ray := range f.Rays {
		if ray.ID == "" || len(ray.ID) > 64 || ids[ray.ID] || (ray.Axis != "horizontal" && ray.Axis != "vertical") ||
			ray.WarningAtMs <= 0 || ray.WarningAtMs > f.ObservedAtMs+50 || ray.LethalAtMs < ray.WarningAtMs ||
			ray.LethalAtMs > f.ObservedAtMs+10000 ||
			(ray.EndsAtMs != 0 && (ray.EndsAtMs <= ray.LethalAtMs || ray.EndsAtMs <= f.ObservedAtMs || ray.EndsAtMs-f.ObservedAtMs > 10000)) {
			return errors.New("invalid ray or absolute deadlines")
		}
		ids[ray.ID] = true
		if (ray.Offset == nil) != (ray.HalfWidth == nil) || (f.GeometryComplete && ray.Offset == nil) {
			return errors.New("incomplete ray geometry")
		}
		if ray.Offset != nil && (!finiteBrontasNumber(*ray.Offset) || math.Abs(*ray.Offset) > 20 ||
			!finiteBrontasNumber(*ray.HalfWidth) || *ray.HalfWidth <= 0 || *ray.HalfWidth > 20) {
			return errors.New("invalid ray geometry")
		}
	}
	return nil
}

// A route includes reaction time, the swept player disk, and holding at the
// destination until every supplied ray ends. Crossing a warning before its
// guarded activation is allowed; checking only the endpoint is insufficient.
func brontasRouteClear(f brontasGuideFrame, target brontasPoint, nowMs int64) bool {
	p := f.Player
	if p == nil || math.Hypot(target.X, target.Y) > 1-p.Radius-.03 {
		return false
	}
	depart := float64(nowMs + brontasGuideReactionMs)
	arrive := depart + math.Hypot(target.X-p.X, target.Y-p.Y)/p.Speed*1000
	for _, ray := range f.Rays {
		if ray.Offset == nil || ray.HalfWidth == nil {
			return false
		}
		start, finish := float64(ray.LethalAtMs-brontasGuideGuardMs), math.Inf(1)
		if ray.EndsAtMs > 0 {
			finish = float64(ray.EndsAtMs + brontasGuideGuardMs)
		}
		if finish <= float64(nowMs) {
			continue
		}
		a, b := p.X, target.X
		if ray.Axis == "horizontal" {
			a, b = p.Y, target.Y
		}
		width := *ray.HalfWidth + p.Radius + .03
		low, high := *ray.Offset-width, *ray.Offset+width
		inside := func(v float64) bool { return v >= low && v <= high }
		if inside(a) && start <= depart && finish >= float64(nowMs) {
			return false
		}
		if inside(b) && finish >= arrive {
			return false
		}
		if math.Abs(b-a) < 1e-9 {
			continue
		}
		u, v := (low-a)/(b-a), (high-a)/(b-a)
		if u > v {
			u, v = v, u
		}
		u, v = math.Max(0, u), math.Min(1, v)
		if u <= v && depart+u*(arrive-depart) <= finish && depart+v*(arrive-depart) >= start {
			return false
		}
	}
	return true
}

// The last safe decision time can precede the snapshot timeout. Otherwise a
// route calculated just in time could remain visible after it is too late to
// start it (notably during later, faster waves).
func brontasRouteValidUntil(f brontasGuideFrame, target brontasPoint, nowMs int64) int64 {
	until := f.ObservedAtMs + brontasGuideFreshMs
	p := f.Player
	travel := math.Hypot(target.X-p.X, target.Y-p.Y) / p.Speed * 1000
	for _, ray := range f.Rays {
		a, b := p.X, target.X
		if ray.Axis == "horizontal" {
			a, b = p.Y, target.Y
		}
		if ray.Offset == nil || ray.HalfWidth == nil || math.Abs(b-a) < 1e-9 {
			continue
		}
		width := *ray.HalfWidth + p.Radius + .03
		u, v := (*ray.Offset-width-a)/(b-a), (*ray.Offset+width-a)/(b-a)
		if u > v {
			u, v = v, u
		}
		u, v = math.Max(0, u), math.Min(1, v)
		exitDelay := float64(brontasGuideReactionMs) + v*travel
		if u <= v && float64(nowMs)+exitDelay <= float64(ray.LethalAtMs-brontasGuideGuardMs) {
			until = min(until, int64(math.Floor(float64(ray.LethalAtMs-brontasGuideGuardMs)-exitDelay)))
		}
	}
	return until
}

func planBrontasGuide(f brontasGuideFrame, nowMs int64) *nativeBrontasGuideView {
	v := &nativeBrontasGuideView{Status: "waiting", Instruction: "等待射线数据", Detail: "按实际落石选择点位", Clock: f.Clock, ValidUntilMs: f.ObservedAtMs + brontasGuideFreshMs}
	if !f.Active || f.ObservedAtMs > nowMs+50 || nowMs >= v.ValidUntilMs {
		return v
	}
	for _, ray := range f.Rays {
		if ray.EndsAtMs > 0 && ray.EndsAtMs+brontasGuideGuardMs <= nowMs {
			continue
		}
		v.Rays = append(v.Rays, ray)
		if v.LethalAtMs == 0 || ray.LethalAtMs < v.LethalAtMs {
			v.LethalAtMs = ray.LethalAtMs
		}
	}
	if len(v.Rays) == 0 {
		v.Status, v.Instruction, v.Detail = "watch", "观察下一条射线", "尚无活动射线；保持观察"
		return v
	}
	horizontal, vertical := false, false
	for _, ray := range v.Rays {
		horizontal = horizontal || ray.Axis == "horizontal"
		vertical = vertical || ray.Axis == "vertical"
	}
	v.Status, v.Detail = "axis", "方向提示；未确定安全落点"
	switch {
	case horizontal && vertical:
		v.Instruction = "横纵交叉 · 观察空隙"
	case horizontal:
		v.Instruction = "横向射线 · 沿地图纵向躲避"
	default:
		v.Instruction = "纵向射线 · 沿地图横向躲避"
	}
	if !f.GeometryComplete || f.Player == nil {
		return v
	}
	v.Player = f.Player
	if math.Hypot(f.Player.X, f.Player.Y) > 1-f.Player.Radius-.03 {
		v.Status, v.Instruction, v.Detail = "enter", "先进入已分配的落石区", "区外路线尚未校准；观察射线入场"
		return v
	}
	f.Rays = v.Rays
	if brontasRouteClear(f, f.Player.brontasPoint, nowMs) {
		v.Status, v.Instruction, v.Detail = "hold", "保持当前位置", "当前已识别射线无需移位"
		return v
	}
	best, cost := brontasPoint{}, math.Inf(1)
	// Deterministic candidates avoid alternating sides for identical inputs.
	for x := -8; x <= 8; x++ {
		for y := -8; y <= 8; y++ {
			q := brontasPoint{X: float64(x) / 10, Y: float64(y) / 10}
			d := math.Hypot(q.X-f.Player.X, q.Y-f.Player.Y)
			if d < cost && brontasRouteClear(f, q, nowMs) {
				best, cost = q, d
			}
		}
	}
	if math.IsInf(cost, 1) {
		v.Status, v.Instruction, v.Detail = "blocked", "未找到可及时到达的落点", "勿跟随旧箭头；观察实际空隙"
		return v
	}
	v.Status, v.Instruction, v.Detail, v.Target = "move", "沿箭头移至圆点", "已检查途中射线；落点位于落石区内", &best
	v.ValidUntilMs = brontasRouteValidUntil(f, best, nowMs)
	return v
}

func rotateBrontasPoint(p brontasPoint, rotation int) brontasPoint {
	for turn := 0; turn < rotation/90; turn++ {
		p = brontasPoint{X: p.Y, Y: -p.X}
	}
	return p
}

func rotateBrontasGuide(v *nativeBrontasGuideView, rotation int) {
	if v.Clock != 0 {
		v.Clock = (v.Clock+rotation/30-1)%12 + 1
	}
	if v.Player != nil {
		p := *v.Player
		p.brontasPoint = rotateBrontasPoint(p.brontasPoint, rotation)
		v.Player = &p
	}
	if v.Target != nil {
		p := rotateBrontasPoint(*v.Target, rotation)
		v.Target = &p
	}
	for i := range v.Rays {
		ray := &v.Rays[i]
		for turn := 0; turn < rotation/90; turn++ {
			if ray.Axis == "horizontal" {
				ray.Axis = "vertical"
			} else {
				ray.Axis = "horizontal"
				if ray.Offset != nil {
					offset := -*ray.Offset
					ray.Offset = &offset
				}
			}
		}
	}
	if v.Status == "axis" && rotation%180 != 0 {
		v.Instruction = strings.NewReplacer("横向射线 · 沿地图纵向躲避", "纵向射线 · 沿地图横向躲避", "纵向射线 · 沿地图横向躲避", "横向射线 · 沿地图纵向躲避").Replace(v.Instruction)
	}
}

func (r *nativeReminderRuntime) brontasGuideView(nowMs int64) *nativeBrontasGuideView {
	f := r.brontasGuide
	matched := false
	for _, rock := range r.brontasRocks {
		if rock.BossID == f.BossID && (f.Clock == 0 || rock.Clock == f.Clock) && f.ObservedAtMs >= rock.AtMs {
			matched = true
		}
	}
	if !matched {
		f = brontasGuideFrame{}
	}
	v := planBrontasGuide(f, nowMs)
	if !matched || !f.Active || nowMs >= f.ObservedAtMs+brontasGuideFreshMs {
		v.ValidUntilMs = nowMs + brontasGuideFreshMs
		v.Detail = "射线识别未接入或数据已过期"
	}
	rotateBrontasGuide(v, r.settings.BossMechanics.BrontasRocks.Rotation)
	return v
}

// Feed the same runtime used by the desktop overlay. A future verified packet
// decoder can call this adapter without changing the renderer or planner.
func submitBrontasGuideFrame(f brontasGuideFrame) bool {
	nativeReminderRuntimeHolder.RLock()
	r := nativeReminderRuntimeHolder.runtime
	nativeReminderRuntimeHolder.RUnlock()
	if r == nil {
		return false
	}
	select {
	case r.brontasGuideCh <- f:
		return true
	default:
		return false
	}
}

func handleBrontasGuide(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if req.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", 405)
		return
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		http.Error(w, "loopback only", 403)
		return
	}
	if origin := req.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Scheme != "http" || u.Host != req.Host {
			http.Error(w, "foreign origin", 403)
			return
		}
	}
	if !strings.HasPrefix(req.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "JSON required", 415)
		return
	}
	var f brontasGuideFrame
	d := json.NewDecoder(http.MaxBytesReader(w, req.Body, 32768))
	d.DisallowUnknownFields()
	if err := d.Decode(&f); err != nil {
		http.Error(w, "invalid observation", 400)
		return
	}
	if d.Decode(new(any)) != io.EOF {
		http.Error(w, "one JSON snapshot required", 400)
		return
	}
	if err := validateBrontasGuideFrame(f, time.Now().UnixMilli()); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if !submitBrontasGuideFrame(f) {
		http.Error(w, "reminder runtime unavailable", 503)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
