package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	battleRecordDefaultPageSize = 10
	battleRecordMaximumPageSize = 50
	battleRecordFavoritesFile   = ".battle_record_favorites.json"
	battleRecordBossHealthFloor = 100_000_000
)

type battleRecordPlayerDPS struct {
	Name        string  `json:"name"`
	DPS         float64 `json:"dps"`
	TotalDamage float64 `json:"totalDamage"`
}

type battleRecordListItem struct {
	Name          string                  `json:"name"`
	Date          string                  `json:"date"`
	TimeRange     string                  `json:"timeRange"`
	StartedAt     int64                   `json:"startedAt"`
	EndedAt       int64                   `json:"endedAt"`
	Size          int64                   `json:"size"`
	EventCount    int                     `json:"eventCount"`
	DamageCount   int                     `json:"damageCount"`
	SkillCount    int                     `json:"skillCount"`
	AttackerCount int                     `json:"attackerCount"`
	TargetCount   int                     `json:"targetCount"`
	TotalDamage   float64                 `json:"totalDamage"`
	Summary       string                  `json:"summary"`
	Active        bool                    `json:"active"`
	Favorite      bool                    `json:"favorite"`
	RelatedFiles  []string                `json:"relatedFiles"`
	Players       []battleRecordPlayerDPS `json:"players"`
	DPSTargetName string                  `json:"dpsTargetName"`
	DPSTargetRace uint32                  `json:"dpsTargetRaceId"`
}

type battleRecordSummaryCacheEntry struct {
	size       int64
	modifiedAt int64
	item       battleRecordListItem
}

type battleRecordEventSummary struct {
	EventID  int                       `json:"EventId"`
	At       int64                     `json:"At"`
	ID       string                    `json:"Id"`
	TargetID string                    `json:"TargetId"`
	SkillID  uint32                    `json:"SkillId"`
	Damage   float64                   `json:"Damage"`
	Name     string                    `json:"Name"`
	RaceID   uint32                    `json:"RaceId"`
	OwnerID  string                    `json:"OwnerId"`
	Stats    []battleRecordStatSummary `json:"Stats"`
}

type battleRecordStatSummary struct {
	StatID int     `json:"StatId"`
	Value  float64 `json:"Value"`
}

type battleRecordActorSummary struct {
	Name          string
	RaceID        uint32
	OwnerID       string
	MaximumHealth float64
}

type battleRecordDamageSummary struct {
	At       int64
	ID       string
	TargetID string
	SkillID  uint32
	Damage   float64
}

type battleRecordTargetDPSWindow struct {
	targetID      string
	startAt       int64
	endAt         int64
	totalDamage   float64
	playerDamages map[string]float64
}

type battleRecordCandidate struct {
	name      string
	info      os.FileInfo
	startedAt int64
	favorite  bool
}

var (
	battleRecordSummaryMu    sync.Mutex
	battleRecordSummaryCache = map[string]battleRecordSummaryCacheEntry{}
	battleRecordFavoritesMu  sync.Mutex
)

func handleBattleRecords(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeBattleRecordError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		serveBattleRecordList(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		serveBattleRecordFile(w, r, name)
	case http.MethodPatch:
		setBattleRecordFavorite(w, r, name)
	case http.MethodDelete:
		deleteBattleRecord(w, name)
	default:
		w.Header().Set("Allow", strings.Join([]string{http.MethodGet, http.MethodPatch, http.MethodDelete}, ", "))
		writeBattleRecordError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
	}
}

func serveBattleRecordList(w http.ResponseWriter, r *http.Request) {
	offset, err := battleRecordQueryInt(r, "offset", 0, 0, int(^uint(0)>>1))
	if err != nil {
		writeBattleRecordError(w, http.StatusBadRequest, err)
		return
	}
	limit, err := battleRecordQueryInt(r, "limit", battleRecordDefaultPageSize, 1, battleRecordMaximumPageSize)
	if err != nil {
		writeBattleRecordError(w, http.StatusBadRequest, err)
		return
	}
	items, total, err := listBattleRecordPage(_logDir, offset, limit)
	if err != nil {
		writeBattleRecordError(w, http.StatusInternalServerError, fmt.Errorf("读取历史记录失败：%w", err))
		return
	}
	nextOffset := offset + len(items)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"records":    items,
		"total":      total,
		"offset":     offset,
		"nextOffset": nextOffset,
		"hasMore":    nextOffset < total,
	})
}

func listBattleRecords(dir string) ([]battleRecordListItem, error) {
	items, _, err := listBattleRecordPage(dir, 0, int(^uint(0)>>1))
	return items, err
}

func listBattleRecordPage(dir string, offset, limit int) ([]battleRecordListItem, int, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []battleRecordListItem{}, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}

	favorites, err := readBattleRecordFavorites(dir)
	if err != nil {
		return nil, 0, err
	}
	candidates := make([]battleRecordCandidate, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !isBattleRecordLogName(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, 0, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		startedAt := info.ModTime().Unix()
		if parsed, ok := battleRecordTimeFromName(entry.Name()); ok {
			startedAt = parsed.Unix()
		}
		candidates = append(candidates, battleRecordCandidate{
			name: entry.Name(), info: info, startedAt: startedAt, favorite: favorites[entry.Name()],
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].favorite != candidates[j].favorite {
			return candidates[i].favorite
		}
		if candidates[i].startedAt != candidates[j].startedAt {
			return candidates[i].startedAt > candidates[j].startedAt
		}
		return candidates[i].name > candidates[j].name
	})

	total := len(candidates)
	if offset >= total {
		return []battleRecordListItem{}, total, nil
	}
	end := offset + limit
	if end > total || end < offset {
		end = total
	}
	items := make([]battleRecordListItem, 0, end-offset)
	for _, candidate := range candidates[offset:end] {
		path := filepath.Join(dir, candidate.name)
		item, err := summarizeBattleRecord(path, candidate.name, candidate.info)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", candidate.name, err)
		}
		item.Active = strings.EqualFold(candidate.name, packetLogFilename)
		item.Favorite = candidate.favorite
		item.RelatedFiles = battleRecordRelatedFiles(dir, candidate.name)
		items = append(items, item)
	}
	return items, total, nil
}

func summarizeBattleRecord(path, name string, info os.FileInfo) (battleRecordListItem, error) {
	cacheKey, err := filepath.Abs(path)
	if err != nil {
		cacheKey = path
	}
	battleRecordSummaryMu.Lock()
	cached, found := battleRecordSummaryCache[cacheKey]
	battleRecordSummaryMu.Unlock()
	if found && cached.size == info.Size() && cached.modifiedAt == info.ModTime().UnixNano() {
		return cached.item, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return battleRecordListItem{}, err
	}
	defer file.Close()

	item := battleRecordListItem{Name: name, Size: info.Size()}
	attackers := map[string]struct{}{}
	targets := map[string]struct{}{}
	skills := map[uint32]struct{}{}
	actors := map[string]battleRecordActorSummary{}
	damages := make([]battleRecordDamageSummary, 0, 1024)
	reader := bufio.NewReaderSize(file, 256*1024)
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			var event battleRecordEventSummary
			if json.Unmarshal(line, &event) == nil {
				item.EventCount++
				if event.ID != "" && event.EventID == 1 {
					actor := actors[event.ID]
					actor.Name = event.Name
					actor.RaceID = event.RaceID
					actor.OwnerID = event.OwnerID
					actors[event.ID] = actor
				}
				if event.ID != "" && event.EventID == 17 {
					actor := actors[event.ID]
					for _, stat := range event.Stats {
						if stat.StatID == 30 && stat.Value > actor.MaximumHealth {
							actor.MaximumHealth = stat.Value
						}
					}
					actors[event.ID] = actor
				}
				if event.At > 0 && (item.StartedAt == 0 || event.At < item.StartedAt) {
					item.StartedAt = event.At
				}
				if event.At > item.EndedAt {
					item.EndedAt = event.At
				}
				if event.EventID == 3 && event.Damage > 0 {
					item.DamageCount++
					item.TotalDamage += event.Damage
					if event.ID != "" {
						attackers[event.ID] = struct{}{}
					}
					if event.TargetID != "" {
						targets[event.TargetID] = struct{}{}
					}
					if event.SkillID > 0 {
						skills[event.SkillID] = struct{}{}
					}
					damages = append(damages, battleRecordDamageSummary{
						At: event.At, ID: event.ID, TargetID: event.TargetID,
						SkillID: event.SkillID, Damage: event.Damage,
					})
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return battleRecordListItem{}, readErr
		}
	}

	item.AttackerCount = len(attackers)
	item.TargetCount = len(targets)
	item.SkillCount = len(skills)
	item.Players, item.DPSTargetRace, item.DPSTargetName = summarizeBattleRecordPlayers(actors, damages)
	if item.StartedAt == 0 {
		if fallback, ok := battleRecordTimeFromName(name); ok {
			item.StartedAt = fallback.Unix()
			item.EndedAt = item.StartedAt
		} else {
			item.StartedAt = info.ModTime().Unix()
			item.EndedAt = item.StartedAt
		}
	}
	start := time.Unix(item.StartedAt, 0).In(time.Local)
	end := time.Unix(max(item.EndedAt, item.StartedAt), 0).In(time.Local)
	item.Date = start.Format("2006-01-02")
	item.TimeRange = fmt.Sprintf("%s - %s", start.Format("15:04:05"), end.Format("15:04:05"))
	if item.DamageCount > 0 {
		item.Summary = fmt.Sprintf("%d 条伤害 · %d 个技能 · %d 名攻击者 / %d 个目标", item.DamageCount, item.SkillCount, item.AttackerCount, item.TargetCount)
	} else if item.EventCount > 0 {
		item.Summary = fmt.Sprintf("无伤害记录 · %d 条状态或场景数据", item.EventCount)
	} else {
		item.Summary = "空记录"
	}

	battleRecordSummaryMu.Lock()
	battleRecordSummaryCache[cacheKey] = battleRecordSummaryCacheEntry{
		size: info.Size(), modifiedAt: info.ModTime().UnixNano(), item: item,
	}
	battleRecordSummaryMu.Unlock()
	return item, nil
}

func summarizeBattleRecordPlayers(
	actors map[string]battleRecordActorSummary,
	damages []battleRecordDamageSummary,
) ([]battleRecordPlayerDPS, uint32, string) {
	targets := map[string]*battleRecordTargetDPSWindow{}
	for _, damage := range damages {
		if damage.TargetID == "" {
			continue
		}
		window := targets[damage.TargetID]
		if window == nil {
			window = &battleRecordTargetDPSWindow{targetID: damage.TargetID, playerDamages: map[string]float64{}}
			targets[damage.TargetID] = window
		}
		window.totalDamage += damage.Damage
		if playerID := battleRecordPlayerID(actors, damage); playerID != "" {
			window.playerDamages[playerID] += damage.Damage
		}
		if damage.At > 0 && (window.startAt == 0 || damage.At < window.startAt) {
			window.startAt = damage.At
		}
		if damage.At > window.endAt {
			window.endAt = damage.At
		}
	}

	selectedTargets, selectedRace, selectedName := selectBattleRecordDPSTargets(actors, targets)
	if len(selectedTargets) == 0 {
		return []battleRecordPlayerDPS{}, 0, ""
	}
	bestByName := map[string]battleRecordPlayerDPS{}
	for _, window := range selectedTargets {
		duration := float64(window.endAt - window.startAt)
		if duration <= 0 {
			duration = 1
		}
		for playerID, total := range window.playerDamages {
			name := strings.TrimSpace(actors[playerID].Name)
			if name == "" {
				name = "未命名玩家"
			}
			candidate := battleRecordPlayerDPS{Name: name, DPS: total / duration, TotalDamage: total}
			if previous, found := bestByName[name]; !found || candidate.DPS > previous.DPS {
				bestByName[name] = candidate
			}
		}
	}
	players := make([]battleRecordPlayerDPS, 0, len(bestByName))
	for _, player := range bestByName {
		players = append(players, player)
	}
	sort.Slice(players, func(i, j int) bool {
		if players[i].DPS != players[j].DPS {
			return players[i].DPS > players[j].DPS
		}
		return players[i].Name < players[j].Name
	})
	if len(players) > 6 {
		players = players[:6]
	}
	return players, selectedRace, selectedName
}

func selectBattleRecordDPSTargets(
	actors map[string]battleRecordActorSummary,
	targets map[string]*battleRecordTargetDPSWindow,
) ([]*battleRecordTargetDPSWindow, uint32, string) {
	type bossPriority struct {
		raceIDs     []uint32
		displayRace uint32
		displayName string
	}
	priorities := []bossPriority{
		{raceIDs: []uint32{7615}, displayRace: 7615, displayName: "雷内恩的米耶尔：悔恨"},
		{raceIDs: []uint32{7603}, displayRace: 7603, displayName: "雷内恩的米耶尔"},
		{raceIDs: []uint32{7602}, displayRace: 7602, displayName: "布隆塔纳斯"},
		{raceIDs: []uint32{7600, 7601}, displayRace: 7600, displayName: "枯木之佩塔克"},
	}
	for _, priority := range priorities {
		matched := make([]*battleRecordTargetDPSWindow, 0)
		for targetID, window := range targets {
			if len(window.playerDamages) == 0 {
				continue
			}
			actor := actors[targetID]
			if battleRecordRaceIn(actor.RaceID, priority.raceIDs) ||
				battleRecordBossNameMatches(actor.Name, priority.displayName) {
				matched = append(matched, window)
			}
		}
		if len(matched) == 0 {
			continue
		}
		sort.Slice(matched, func(i, j int) bool { return matched[i].targetID < matched[j].targetID })
		return matched, priority.displayRace, priority.displayName
	}

	var selected *battleRecordTargetDPSWindow
	var selectedHealth float64
	for targetID, window := range targets {
		if len(window.playerDamages) == 0 {
			continue
		}
		actor := actors[targetID]
		estimatedHealth := actor.MaximumHealth
		if estimatedHealth <= 0 {
			estimatedHealth = window.totalDamage
		}
		if estimatedHealth < battleRecordBossHealthFloor {
			continue
		}
		if selected == nil ||
			estimatedHealth > selectedHealth ||
			(estimatedHealth == selectedHealth && window.totalDamage > selected.totalDamage) ||
			(estimatedHealth == selectedHealth && window.totalDamage == selected.totalDamage && window.endAt > selected.endAt) ||
			(estimatedHealth == selectedHealth && window.totalDamage == selected.totalDamage && window.endAt == selected.endAt && targetID < selected.targetID) {
			selected = window
			selectedHealth = estimatedHealth
		}
	}
	if selected == nil {
		return nil, 0, ""
	}
	actor := actors[selected.targetID]
	name := strings.TrimSpace(actor.Name)
	if name == "" {
		name = fmt.Sprintf("Boss 目标 %d", actor.RaceID)
	}
	return []*battleRecordTargetDPSWindow{selected}, actor.RaceID, name
}

func battleRecordRaceIn(raceID uint32, choices []uint32) bool {
	for _, choice := range choices {
		if raceID == choice {
			return true
		}
	}
	return false
}

func battleRecordBossNameMatches(rawName, displayName string) bool {
	name := strings.TrimSpace(rawName)
	switch displayName {
	case "雷内恩的米耶尔：悔恨":
		return strings.Contains(name, "雷内恩的米耶尔") && strings.Contains(name, "悔恨")
	case "雷内恩的米耶尔":
		return strings.Contains(name, "雷内恩的米耶尔") && !strings.Contains(name, "悔恨")
	case "布隆塔纳斯":
		return strings.Contains(name, "布隆塔纳斯")
	case "枯木之佩塔克":
		return strings.Contains(name, "枯木") && strings.Contains(name, "佩塔克")
	default:
		return false
	}
}

func battleRecordPlayerID(
	actors map[string]battleRecordActorSummary,
	damage battleRecordDamageSummary,
) string {
	actor, ok := actors[damage.ID]
	if !ok {
		return ""
	}
	if battleRecordPCRace(actor.RaceID) {
		return damage.ID
	}
	if actor.OwnerID != "" && battleRecordMarionetteSkill(damage.SkillID) {
		owner, found := actors[actor.OwnerID]
		if found && battleRecordPCRace(owner.RaceID) {
			return actor.OwnerID
		}
	}
	return ""
}

func battleRecordPCRace(raceID uint32) bool {
	switch raceID {
	case 8001, 8002, 9001, 9002, 10001, 10002:
		return true
	default:
		return false
	}
}

func battleRecordMarionetteSkill(skillID uint32) bool {
	return (skillID >= 54101 && skillID <= 54106) ||
		(skillID >= 54151 && skillID <= 54156) ||
		(skillID >= 59167 && skillID <= 59169)
}

func battleRecordQueryInt(r *http.Request, name string, fallback, minimum, maximum int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("参数 %s 不正确", name)
	}
	return value, nil
}

func setBattleRecordFavorite(w http.ResponseWriter, r *http.Request, name string) {
	if _, err := battleRecordFileInfo(_logDir, name); err != nil {
		writeBattleRecordFileError(w, err)
		return
	}
	var payload struct {
		Favorite *bool `json:"favorite"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
	if err := decoder.Decode(&payload); err != nil || payload.Favorite == nil {
		writeBattleRecordError(w, http.StatusBadRequest, errors.New("收藏设置不正确"))
		return
	}
	battleRecordFavoritesMu.Lock()
	defer battleRecordFavoritesMu.Unlock()
	favorites, err := readBattleRecordFavoritesUnlocked(_logDir)
	if err != nil {
		writeBattleRecordError(w, http.StatusInternalServerError, errors.New("无法读取收藏设置"))
		return
	}
	if *payload.Favorite {
		favorites[name] = true
	} else {
		delete(favorites, name)
	}
	if err := writeBattleRecordFavoritesUnlocked(_logDir, favorites); err != nil {
		writeBattleRecordError(w, http.StatusInternalServerError, errors.New("无法保存收藏设置"))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"favorite": *payload.Favorite})
}

func deleteBattleRecord(w http.ResponseWriter, name string) {
	if _, err := battleRecordFileInfo(_logDir, name); err != nil {
		writeBattleRecordFileError(w, err)
		return
	}
	if strings.EqualFold(name, packetLogFilename) {
		writeBattleRecordError(w, http.StatusConflict, errors.New("本次运行的记录不能删除"))
		return
	}
	files := battleRecordRelatedFiles(_logDir, name)
	deleted := make([]string, 0, len(files))
	for _, relatedName := range files {
		path := filepath.Join(_logDir, relatedName)
		if err := os.Remove(path); err != nil {
			writeBattleRecordError(w, http.StatusInternalServerError, fmt.Errorf("删除 %s 失败，已删除 %d 个文件", relatedName, len(deleted)))
			return
		}
		deleted = append(deleted, relatedName)
	}

	battleRecordSummaryMu.Lock()
	for cacheKey := range battleRecordSummaryCache {
		if strings.EqualFold(filepath.Base(cacheKey), name) {
			delete(battleRecordSummaryCache, cacheKey)
		}
	}
	battleRecordSummaryMu.Unlock()

	battleRecordFavoritesMu.Lock()
	favorites, err := readBattleRecordFavoritesUnlocked(_logDir)
	if err == nil {
		delete(favorites, name)
		err = writeBattleRecordFavoritesUnlocked(_logDir, favorites)
	}
	battleRecordFavoritesMu.Unlock()
	if err != nil {
		writeBattleRecordError(w, http.StatusInternalServerError, errors.New("记录已删除，但收藏设置更新失败"))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"deleted": deleted})
}

func readBattleRecordFavorites(dir string) (map[string]bool, error) {
	battleRecordFavoritesMu.Lock()
	defer battleRecordFavoritesMu.Unlock()
	return readBattleRecordFavoritesUnlocked(dir)
}

func readBattleRecordFavoritesUnlocked(dir string) (map[string]bool, error) {
	data, err := os.ReadFile(filepath.Join(dir, battleRecordFavoritesFile))
	if errors.Is(err, os.ErrNotExist) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	favorites := map[string]bool{}
	if len(bytes.TrimSpace(data)) == 0 {
		return favorites, nil
	}
	if err := json.Unmarshal(data, &favorites); err != nil {
		return nil, err
	}
	for name, favorite := range favorites {
		if !favorite || !isBattleRecordLogName(name) || filepath.Base(name) != name {
			delete(favorites, name)
		}
	}
	return favorites, nil
}

func writeBattleRecordFavoritesUnlocked(dir string, favorites map[string]bool) error {
	data, err := json.Marshal(favorites)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, battleRecordFavoritesFile), data, 0o600)
}

func battleRecordRelatedFiles(dir, name string) []string {
	if !isBattleRecordLogName(name) || filepath.Base(name) != name {
		return []string{}
	}
	stamp := strings.TrimSuffix(strings.TrimPrefix(name, "packet_log_"), ".ndjson")
	candidates := []string{name, "log_" + stamp + ".txt"}
	if startedAt, ok := battleRecordTimeFromName(name); ok {
		candidates = append(candidates, fmt.Sprintf("packet_capture_%d.pcapng", startedAt.Unix()))
	}
	related := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		info, err := os.Lstat(filepath.Join(dir, candidate))
		if err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			related = append(related, candidate)
		}
	}
	return related
}

func battleRecordFileInfo(dir, name string) (os.FileInfo, error) {
	if !isBattleRecordLogName(name) || filepath.Base(name) != name {
		return nil, fmt.Errorf("bad_name")
	}
	info, err := os.Lstat(filepath.Join(dir, name))
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("bad_file")
	}
	return info, nil
}

func writeBattleRecordFileError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, os.ErrNotExist):
		writeBattleRecordError(w, http.StatusNotFound, errors.New("历史记录不存在"))
	case err != nil && err.Error() == "bad_name":
		writeBattleRecordError(w, http.StatusBadRequest, errors.New("历史记录名称不正确"))
	default:
		writeBattleRecordError(w, http.StatusBadRequest, errors.New("历史记录无法读取"))
	}
}

func serveBattleRecordFile(w http.ResponseWriter, r *http.Request, name string) {
	info, err := battleRecordFileInfo(_logDir, name)
	if err != nil {
		writeBattleRecordFileError(w, err)
		return
	}
	path := filepath.Join(_logDir, name)
	file, err := os.Open(path)
	if err != nil {
		writeBattleRecordError(w, http.StatusInternalServerError, errors.New("历史记录无法打开"))
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	http.ServeContent(w, r, name, info.ModTime(), file)
}

func isBattleRecordLogName(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasPrefix(lower, "packet_log_") && strings.HasSuffix(lower, ".ndjson")
}

func battleRecordTimeFromName(name string) (time.Time, bool) {
	stamp := strings.TrimSuffix(strings.TrimPrefix(strings.ToLower(name), "packet_log_"), ".ndjson")
	value, err := time.ParseInLocation("2006-01-02_15-04-05", stamp, time.Local)
	return value, err == nil
}

func writeBattleRecordError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
