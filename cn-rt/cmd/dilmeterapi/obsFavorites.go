package main

// OBS authorization reuses saved healer favorites, but never trusts the healer
// monitor's name-only runtime rebinding. Its own exact identity pins live beside
// healer-monitor.json; the original favorite settings are never rewritten here.
import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

type obsFavoriteBinding struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	LocalID     string `json:"localId"`
	ServerScope string `json:"serverScope"`
}
type obsFavoriteMember struct {
	obsFavoriteBinding
	Ready    bool   `json:"ready"`
	Observed bool   `json:"observed"`
	Reason   string `json:"reason,omitempty"`
}
type obsFavoriteSnapshot struct {
	Version     int                 `json:"version"`
	LocalID     string              `json:"localId"`
	ServerScope string              `json:"serverScope"`
	Members     []obsFavoriteMember `json:"members"`
	Error       string              `json:"error,omitempty"`
}
type obsFavoriteStore struct {
	path                            string
	bindings                        map[string]obsFavoriteBinding
	players                         map[string]string
	verified                        map[string]string
	localID, serverScope, loadError string
}

func newObsFavoriteStore(path string) *obsFavoriteStore {
	s := &obsFavoriteStore{path: path, bindings: map[string]obsFavoriteBinding{}, players: map[string]string{}, verified: map[string]string{}}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s
	}
	if err != nil || json.Unmarshal(data, &s.bindings) != nil || s.bindings == nil {
		s.loadError = "角色绑定文件无法读取；保留原文件，暂停队友动画"
	}
	return s
}
func obsPlayerID(id string) bool {
	n, err := strconv.ParseUint(id, 10, 64)
	return err == nil && n > 0 && strconv.FormatUint(n, 10) == id
}
func (s *obsFavoriteStore) onEvent(current event.IEvent) {
	switch v := current.(type) {
	case *event.EventLocalEntity:
		if v.Reset || s.localID != "" && s.localID != v.Id || s.serverScope != "" && s.serverScope != v.ServerScope {
			clear(s.players)
			clear(s.verified)
		}
		s.localID, s.serverScope = "", ""
		if net.ParseIP(v.ServerScope) != nil {
			s.serverScope = v.ServerScope
			if v.Reliable && obsPlayerID(v.Id) {
				s.localID = v.Id
			}
		}
	case *event.EventEntityAppear:
		if obsPlayerID(v.Id) && battleRecordPCRace(v.RaceId) && (v.OwnerId == "" || v.OwnerId == "0") && v.Name != "" {
			if len(s.players) < 512 || s.players[v.Id] != "" {
				s.players[v.Id] = v.Name
				if len(s.verified) < 2048 || s.verified[v.Id] != "" {
					s.verified[v.Id] = v.Name
				}
			}
		} else {
			delete(s.players, v.Id)
			delete(s.verified, v.Id)
		}
	case *event.EventEntityDisappear:
		delete(s.players, v.Id)
	}
}

// Called under healer.mu. Initial migration requires one unique freshly observed
// player. Later sessions must match the pinned endpoint, local ID, peer ID AND
// exact name. A server/address or ID change never silently migrates a favorite.
func (s *obsFavoriteStore) snapshot(choices []healerMemberSelection) obsFavoriteSnapshot {
	result := obsFavoriteSnapshot{Version: 1, LocalID: s.localID, ServerScope: s.serverScope, Members: []obsFavoriteMember{}, Error: s.loadError}
	if s.loadError != "" {
		return result
	}
	names := map[string]bool{}
	for _, member := range choices {
		if member.Favorite && member.Name != "" {
			names[member.Name] = true
		}
	}
	next := map[string]obsFavoriteBinding{}
	changed := false
	for name, binding := range s.bindings {
		if names[name] {
			next[name] = binding
		} else {
			changed = true
		}
	}
	for name := range names {
		ids := []string{}
		for id, observed := range s.players {
			if id != s.localID && observed == name {
				ids = append(ids, id)
			}
		}
		binding, exists := next[name]
		if !exists && s.localID != "" && s.serverScope != "" && len(ids) == 1 {
			binding = obsFavoriteBinding{Name: name, ID: ids[0], LocalID: s.localID, ServerScope: s.serverScope}
			next[name] = binding
			changed = true
			exists = true
		}
		member := obsFavoriteMember{obsFavoriteBinding: binding, Reason: "等待队友重新出现在现有事件中"}
		member.Name = name
		member.Observed = exists && s.players[binding.ID] == name
		switch {
		case s.localID == "" || s.serverScope == "":
			member.Reason = "等待可靠本机身份及实时服务器地址（离线回放不绑定）"
		case len(ids) > 1:
			member.Reason = "同名角色不唯一，暂停触发"
		case exists && (binding.LocalID != s.localID || binding.ServerScope != s.serverScope):
			member.Reason = "本机角色或服务器地址已改变，保留原绑定并暂停触发"
		case len(ids) == 1 && exists && binding.ID != ids[0]:
			member.Reason = "同名角色 ID 与原绑定不同，暂停触发"
		case exists && s.verified[binding.ID] == name:
			member.Ready = true
			member.Reason = ""
			if !member.Observed {
				member.Reason = "超出可见范围/暂不可见；收到已确认施放才播放"
			}
		}
		result.Members = append(result.Members, member)
	}
	sort.Slice(result.Members, func(i, j int) bool { return result.Members[i].Name < result.Members[j].Name })
	if changed {
		data, err := json.MarshalIndent(next, "", "  ")
		if err == nil {
			err = os.WriteFile(s.path+".tmp", data, 0600)
		}
		if err == nil {
			err = os.Rename(s.path+".tmp", s.path)
		}
		if err != nil {
			result.Error = "无法保存角色绑定；暂停队友动画，原收藏设置未修改"
			for i := range result.Members {
				result.Members[i].Ready = false
			}
		} else {
			s.bindings = next
		}
	}
	return result
}
func handleObsFavorites(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		http.Error(w, "local access only", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	nativeReminderRuntimeHolder.RLock()
	runtime := nativeReminderRuntimeHolder.runtime
	nativeReminderRuntimeHolder.RUnlock()
	if runtime == nil || runtime.healer == nil || runtime.healer.obsFavorites == nil {
		http.Error(w, "favorites unavailable", http.StatusServiceUnavailable)
		return
	}
	h := runtime.healer
	h.mu.Lock()
	defer h.mu.Unlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(h.obsFavorites.snapshot(h.settings.Members))
}
