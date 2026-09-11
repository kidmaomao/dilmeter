package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBattleRecordsListIsNewestFirstAndSummarizesContent(t *testing.T) {
	dir := t.TempDir()
	previousDir := _logDir
	previousFilename := packetLogFilename
	_logDir = dir
	packetLogFilename = "packet_log_2026-08-29_12-00-00.ndjson"
	battleRecordSummaryMu.Lock()
	battleRecordSummaryCache = map[string]battleRecordSummaryCacheEntry{}
	battleRecordSummaryMu.Unlock()
	t.Cleanup(func() {
		_logDir = previousDir
		packetLogFilename = previousFilename
	})

	older := strings.Join([]string{
		`{"EventId":1,"At":1787932800,"Id":"player-a","Name":"A","RaceId":10001}`,
		`{"EventId":3,"At":1787932810,"Id":"player-a","TargetId":"boss-a","SkillId":59145,"Damage":1200}`,
		`{"EventId":3,"At":1787932812,"Id":"player-b","TargetId":"boss-a","SkillId":59104,"Damage":800}`,
	}, "\n") + "\n"
	newer := strings.Join([]string{
		`{"EventId":11,"At":1787976000,"Id":"0","Reset":true}`,
		`{"EventId":17,"At":1787976030,"Id":"player-a","Private":true}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "packet_log_2026-08-29_00-00-00.ndjson"), []byte(older), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, packetLogFilename), []byte(newer), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "log_2026-08-29_12-00-00.txt"), []byte("diagnostic"), 0o644); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/battle_records", nil)
	response := httptest.NewRecorder()
	handleBattleRecords(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var result struct {
		Records []battleRecordListItem `json:"records"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 2 {
		t.Fatalf("records=%#v, want only two packet logs", result.Records)
	}
	if result.Records[0].Name != packetLogFilename || !result.Records[0].Active {
		t.Fatalf("newest active record=%#v", result.Records[0])
	}
	olderItem := result.Records[1]
	if olderItem.DamageCount != 2 || olderItem.SkillCount != 2 || olderItem.AttackerCount != 2 ||
		olderItem.TargetCount != 1 || olderItem.TotalDamage != 2000 {
		t.Fatalf("older summary=%#v", olderItem)
	}
	if !strings.Contains(olderItem.Summary, "2 条伤害") || olderItem.TimeRange == "" || olderItem.Date == "" {
		t.Fatalf("older display summary=%#v", olderItem)
	}
}

func TestBattleRecordsServesOnlyManagedPacketLogs(t *testing.T) {
	dir := t.TempDir()
	previousDir := _logDir
	_logDir = dir
	t.Cleanup(func() { _logDir = previousDir })
	name := "packet_log_2026-08-29_12-00-00.ndjson"
	content := `{"EventId":3,"At":1787976000,"Id":"p","TargetId":"b","SkillId":1,"Damage":10}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/battle_records?name="+name, nil)
	response := httptest.NewRecorder()
	handleBattleRecords(response, request)
	if response.Code != http.StatusOK || response.Body.String() != content {
		t.Fatalf("record response status=%d body=%q", response.Code, response.Body.String())
	}

	unsafeRequest := httptest.NewRequest(http.MethodGet, "/api/battle_records?name=../secret.ndjson", nil)
	unsafeResponse := httptest.NewRecorder()
	handleBattleRecords(unsafeResponse, unsafeRequest)
	if unsafeResponse.Code != http.StatusBadRequest {
		t.Fatalf("unsafe name status=%d body=%s", unsafeResponse.Code, unsafeResponse.Body.String())
	}

	postRequest := httptest.NewRequest(http.MethodPost, "/api/battle_records", nil)
	postResponse := httptest.NewRecorder()
	handleBattleRecords(postResponse, postRequest)
	if postResponse.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status=%d", postResponse.Code)
	}
}

func TestBattleRecordEmptyFileUsesFilenameTime(t *testing.T) {
	dir := t.TempDir()
	name := "packet_log_2026-08-29_12-34-56.ndjson"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	item, err := summarizeBattleRecord(path, name, info)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := time.ParseInLocation("2006-01-02_15-04-05", "2026-08-29_12-34-56", time.Local)
	if item.StartedAt != want.Unix() || item.Summary != "空记录" {
		t.Fatalf("empty item=%#v", item)
	}
}

func TestBattleRecordsPaginatesTenAtATimeAndPinsFavorites(t *testing.T) {
	dir := t.TempDir()
	previousDir := _logDir
	previousFilename := packetLogFilename
	_logDir = dir
	packetLogFilename = ""
	battleRecordSummaryMu.Lock()
	battleRecordSummaryCache = map[string]battleRecordSummaryCacheEntry{}
	battleRecordSummaryMu.Unlock()
	t.Cleanup(func() {
		_logDir = previousDir
		packetLogFilename = previousFilename
	})
	for index := 0; index < 15; index++ {
		name := fmt.Sprintf("packet_log_2026-08-%02d_12-00-00.ndjson", index+1)
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	favoriteName := "packet_log_2026-08-01_12-00-00.ndjson"
	if err := os.WriteFile(filepath.Join(dir, battleRecordFavoritesFile), []byte(`{"`+favoriteName+`":true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/battle_records?offset=0&limit=10", nil)
	response := httptest.NewRecorder()
	handleBattleRecords(response, request)
	var result struct {
		Records []battleRecordListItem `json:"records"`
		Total   int                    `json:"total"`
		HasMore bool                   `json:"hasMore"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || len(result.Records) != 10 || result.Total != 15 || !result.HasMore {
		t.Fatalf("status=%d result=%#v", response.Code, result)
	}
	if result.Records[0].Name != favoriteName || !result.Records[0].Favorite {
		t.Fatalf("favorite was not pinned: %#v", result.Records[0])
	}
	battleRecordSummaryMu.Lock()
	cachedCount := len(battleRecordSummaryCache)
	battleRecordSummaryMu.Unlock()
	if cachedCount != 10 {
		t.Fatalf("first page summarized %d records, want 10", cachedCount)
	}
}

func TestBattleRecordSummaryIncludesPlayerDPSAndExcludesNormalPets(t *testing.T) {
	dir := t.TempDir()
	name := "packet_log_2026-08-29_12-34-56.ndjson"
	content := strings.Join([]string{
		`{"EventId":1,"At":100,"Id":"boss","Name":"Boss","RaceId":7603,"OwnerId":""}`,
		`{"EventId":1,"At":100,"Id":"player","Name":"Alice","RaceId":10001,"OwnerId":""}`,
		`{"EventId":1,"At":100,"Id":"pet","Name":"Pet","RaceId":5001,"OwnerId":"player"}`,
		`{"EventId":3,"At":101,"Id":"player","TargetId":"boss","SkillId":100,"Damage":1000}`,
		`{"EventId":3,"At":102,"Id":"pet","TargetId":"boss","SkillId":200,"Damage":9000}`,
		`{"EventId":3,"At":103,"Id":"pet","TargetId":"boss","SkillId":54101,"Damage":3000}`,
	}, "\n") + "\n"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	item, err := summarizeBattleRecord(path, name, info)
	if err != nil {
		t.Fatal(err)
	}
	if len(item.Players) != 1 || item.Players[0].Name != "Alice" || item.Players[0].TotalDamage != 4000 {
		t.Fatalf("player DPS summary=%#v", item.Players)
	}
	if item.DPSTargetRace != 7603 || item.DPSTargetName != "雷内恩的米耶尔" {
		t.Fatalf("DPS target=%d %q", item.DPSTargetRace, item.DPSTargetName)
	}
}

func TestBattleRecordSummaryUsesPriorityBossAndEachPlayersHighestDPS(t *testing.T) {
	dir := t.TempDir()
	name := "packet_log_2026-08-29_13-00-00.ndjson"
	content := strings.Join([]string{
		`{"EventId":1,"At":90,"Id":"small-target","Name":"Small Target","RaceId":7001,"OwnerId":""}`,
		`{"EventId":1,"At":90,"Id":"miel","Name":"Miel","RaceId":7603,"OwnerId":""}`,
		`{"EventId":1,"At":90,"Id":"remorse-1","Name":"Remorse 1","RaceId":7615,"OwnerId":""}`,
		`{"EventId":1,"At":90,"Id":"remorse-2","Name":"Remorse 2","RaceId":7615,"OwnerId":""}`,
		`{"EventId":1,"At":90,"Id":"alice","Name":"Alice","RaceId":10001,"OwnerId":""}`,
		`{"EventId":1,"At":90,"Id":"bob","Name":"Bob","RaceId":10002,"OwnerId":""}`,
		`{"EventId":3,"At":300,"Id":"alice","TargetId":"small-target","SkillId":100,"Damage":9000}`,
		`{"EventId":3,"At":301,"Id":"bob","TargetId":"small-target","SkillId":100,"Damage":9000}`,
		`{"EventId":3,"At":400,"Id":"alice","TargetId":"miel","SkillId":100,"Damage":5000}`,
		`{"EventId":3,"At":410,"Id":"bob","TargetId":"miel","SkillId":100,"Damage":5000}`,
		`{"EventId":3,"At":100,"Id":"alice","TargetId":"remorse-1","SkillId":100,"Damage":1000}`,
		`{"EventId":3,"At":110,"Id":"bob","TargetId":"remorse-1","SkillId":100,"Damage":2000}`,
		`{"EventId":3,"At":200,"Id":"alice","TargetId":"remorse-2","SkillId":100,"Damage":2000}`,
		`{"EventId":3,"At":205,"Id":"bob","TargetId":"remorse-2","SkillId":100,"Damage":100}`,
	}, "\n") + "\n"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	item, err := summarizeBattleRecord(path, name, info)
	if err != nil {
		t.Fatal(err)
	}
	if len(item.Players) != 2 {
		t.Fatalf("players=%#v", item.Players)
	}
	if item.DPSTargetRace != 7615 || item.DPSTargetName != "雷内恩的米耶尔：悔恨" {
		t.Fatalf("DPS target=%d %q", item.DPSTargetRace, item.DPSTargetName)
	}
	if item.Players[0].Name != "Alice" || item.Players[0].DPS != 400 || item.Players[0].TotalDamage != 2000 {
		t.Fatalf("Alice highest DPS=%#v", item.Players[0])
	}
	if item.Players[1].Name != "Bob" || item.Players[1].DPS != 200 || item.Players[1].TotalDamage != 2000 {
		t.Fatalf("Bob highest DPS=%#v", item.Players[1])
	}
}

func TestBattleRecordDPSTargetPriorityAndBossHealthFallback(t *testing.T) {
	window := func(id string, damage float64) *battleRecordTargetDPSWindow {
		return &battleRecordTargetDPSWindow{
			targetID: id, startAt: 100, endAt: 110, totalDamage: damage,
			playerDamages: map[string]float64{"player": damage},
		}
	}
	actors := map[string]battleRecordActorSummary{
		"remorse": {Name: "Remorse", RaceID: 7615, MaximumHealth: 400_000_000},
		"miel":    {Name: "Miel", RaceID: 7603, MaximumHealth: 300_000_000},
		"brontas": {Name: "Brontas", RaceID: 7602, MaximumHealth: 250_000_000},
		"petak":   {Name: "Petak", RaceID: 7601, MaximumHealth: 200_000_000},
		"other":   {Name: "Other Boss", RaceID: 7002, MaximumHealth: 500_000_000},
		"small":   {Name: "Small Target", RaceID: 7001, MaximumHealth: 50_000_000},
	}
	allTargets := map[string]*battleRecordTargetDPSWindow{
		"remorse": window("remorse", 10), "miel": window("miel", 20),
		"brontas": window("brontas", 30), "petak": window("petak", 40),
		"other": window("other", 50), "small": window("small", 1_000),
	}
	cases := []struct {
		name       string
		remove     []string
		wantRace   uint32
		wantName   string
		wantTarget string
	}{
		{name: "remorse first", wantRace: 7615, wantName: "雷内恩的米耶尔：悔恨", wantTarget: "remorse"},
		{name: "normal Miel second", remove: []string{"remorse"}, wantRace: 7603, wantName: "雷内恩的米耶尔", wantTarget: "miel"},
		{name: "Brontas third", remove: []string{"remorse", "miel"}, wantRace: 7602, wantName: "布隆塔纳斯", wantTarget: "brontas"},
		{name: "Petak fourth", remove: []string{"remorse", "miel", "brontas"}, wantRace: 7600, wantName: "枯木之佩塔克", wantTarget: "petak"},
		{name: "largest other Boss fallback", remove: []string{"remorse", "miel", "brontas", "petak"}, wantRace: 7002, wantName: "Other Boss", wantTarget: "other"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			targets := map[string]*battleRecordTargetDPSWindow{}
			for id, candidate := range allTargets {
				targets[id] = candidate
			}
			for _, id := range test.remove {
				delete(targets, id)
			}
			selected, raceID, name := selectBattleRecordDPSTargets(actors, targets)
			if raceID != test.wantRace || name != test.wantName || len(selected) != 1 || selected[0].targetID != test.wantTarget {
				t.Fatalf("selected=%#v race=%d name=%q", selected, raceID, name)
			}
		})
	}
}

func TestBattleRecordFavoriteAndDeleteRemovesAssociatedFiles(t *testing.T) {
	dir := t.TempDir()
	previousDir := _logDir
	previousFilename := packetLogFilename
	_logDir = dir
	packetLogFilename = "packet_log_2026-08-29_12-00-00.ndjson"
	t.Cleanup(func() {
		_logDir = previousDir
		packetLogFilename = previousFilename
	})

	name := "packet_log_2026-08-28_12-00-00.ndjson"
	startedAt, _ := battleRecordTimeFromName(name)
	related := []string{
		name,
		"log_2026-08-28_12-00-00.txt",
		fmt.Sprintf("packet_capture_%d.pcapng", startedAt.Unix()),
	}
	for _, relatedName := range related {
		if err := os.WriteFile(filepath.Join(dir, relatedName), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "packet_capture_123.pcapng"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	favoriteRequest := httptest.NewRequest(http.MethodPatch, "/api/battle_records?name="+name, bytes.NewBufferString(`{"favorite":true}`))
	favoriteResponse := httptest.NewRecorder()
	handleBattleRecords(favoriteResponse, favoriteRequest)
	if favoriteResponse.Code != http.StatusOK {
		t.Fatalf("favorite status=%d body=%s", favoriteResponse.Code, favoriteResponse.Body.String())
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/battle_records?name="+name, nil)
	deleteResponse := httptest.NewRecorder()
	handleBattleRecords(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}
	for _, relatedName := range related {
		if _, err := os.Stat(filepath.Join(dir, relatedName)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("related file still exists: %s", relatedName)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "packet_capture_123.pcapng")); err != nil {
		t.Fatalf("unrelated capture removed: %v", err)
	}

	activePath := filepath.Join(dir, packetLogFilename)
	if err := os.WriteFile(activePath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	activeRequest := httptest.NewRequest(http.MethodDelete, "/api/battle_records?name="+packetLogFilename, nil)
	activeResponse := httptest.NewRecorder()
	handleBattleRecords(activeResponse, activeRequest)
	if activeResponse.Code != http.StatusConflict {
		t.Fatalf("active delete status=%d body=%s", activeResponse.Code, activeResponse.Body.String())
	}
}
