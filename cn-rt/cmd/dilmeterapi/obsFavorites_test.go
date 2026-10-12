package main

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

func obsIdentity(s *obsFavoriteStore, scope, self string, reset bool) {
	s.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{Id: self}, Reliable: true, Reset: reset, ServerScope: scope})
}
func obsAppear(s *obsFavoriteStore, id, name string) { s.onEvent(liveTestAppear(id, name, 9001, 100)) }
func TestObsFavoritesPersistAndRestoreOnlyExactScopedFreshIdentity(t *testing.T) {
	p := filepath.Join(t.TempDir(), "obs.json")
	s := newObsFavoriteStore(p)
	choices := []healerMemberSelection{{Name: "常用", Favorite: true}, {Name: "路人", Favorite: false}}
	obsIdentity(s, "198.51.100.1", "1", false)
	obsAppear(s, "2", "常用")
	obsAppear(s, "3", "路人")
	if got := s.snapshot(choices); len(got.Members) != 1 || !got.Members[0].Ready {
		t.Fatalf("initial binding: %+v", got)
	}
	for _, restart := range []bool{false, true} {
		if restart {
			s = newObsFavoriteStore(p)
		}
		obsIdentity(s, "198.51.100.1", "1", true)
		if s.snapshot(choices).Members[0].Ready {
			t.Fatal("restored without fresh appearance")
		}
		obsAppear(s, "2", "常用")
		if !s.snapshot(choices).Members[0].Ready {
			t.Fatal("same identity did not restore automatically")
		}
	}
	obsIdentity(s, "198.51.100.2", "1", true)
	obsAppear(s, "2", "常用")
	if s.snapshot(choices).Members[0].Ready {
		t.Fatal("same name and ID from another server authorized")
	}
	obsIdentity(s, "198.51.100.1", "9", true)
	obsAppear(s, "2", "常用")
	if s.snapshot(choices).Members[0].Ready {
		t.Fatal("different local character authorized")
	}
	obsIdentity(s, "198.51.100.1", "1", true)
	obsAppear(s, "4", "常用")
	if s.snapshot(choices).Members[0].Ready {
		t.Fatal("same name with different ID rebound")
	}
	obsAppear(s, "2", "常用")
	if s.snapshot(choices).Members[0].Ready {
		t.Fatal("ambiguous same name authorized")
	}
	s.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{Id: "4"}})
	if !s.snapshot(choices).Members[0].Ready {
		t.Fatal("original identity should restore after ambiguity clears")
	}
	if got := s.snapshot(nil); len(got.Members) != 0 || len(s.bindings) != 0 {
		t.Fatal("favorite deletion not synchronized")
	}
	if len(newObsFavoriteStore(p).bindings) != 0 {
		t.Fatal("deleted binding restored from disk")
	}
}
func TestObsFavoritesOfflinePetsCorruptStorageFailClosed(t *testing.T) {
	p := filepath.Join(t.TempDir(), "obs.json")
	s := newObsFavoriteStore(p)
	choices := []healerMemberSelection{{Name: "常用", Favorite: true}}
	obsIdentity(s, "", "1", false)
	obsAppear(s, "2", "常用")
	if s.snapshot(choices).Members[0].Ready {
		t.Fatal("offline source pinned")
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("offline source wrote bindings")
	}
	obsIdentity(s, "198.51.100.1", "1", true)
	pet := liveTestAppear("2", "常用", 9001, 100)
	pet.OwnerId = "3"
	s.onEvent(pet)
	if s.snapshot(choices).Members[0].Ready {
		t.Fatal("owned creature pinned")
	}
	if err := os.WriteFile(p, []byte("invalid original"), 0600); err != nil {
		t.Fatal(err)
	}
	s = newObsFavoriteStore(p)
	obsIdentity(s, "198.51.100.1", "1", false)
	obsAppear(s, "2", "常用")
	if s.snapshot(choices).Error == "" {
		t.Fatal("corrupt store accepted")
	}
	data, _ := os.ReadFile(p)
	if string(data) != "invalid original" {
		t.Fatal("corrupt original overwritten")
	}
}

func TestObsFavoriteVisibilityDoesNotEraseVerifiedBinding(t *testing.T) {
	s := newObsFavoriteStore(filepath.Join(t.TempDir(), "obs.json"))
	choices := []healerMemberSelection{{Name: "常用", Favorite: true}}
	obsIdentity(s, "198.51.100.1", "1", false)
	obsAppear(s, "2", "常用")
	first := s.snapshot(choices).Members[0]
	if !first.Ready || !first.Observed {
		t.Fatal(first)
	}
	s.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{Id: "2"}})
	absent := s.snapshot(choices).Members[0]
	if !absent.Ready || absent.Observed || absent.ID != "2" || absent.Reason == "" {
		t.Fatal(absent)
	}
	obsAppear(s, "2", "常用")
	if !s.snapshot(choices).Members[0].Observed {
		t.Fatal("return did not restore visibility")
	}
	pet := liveTestAppear("2", "常用", 9001, 100)
	pet.OwnerId = "3"
	s.onEvent(pet)
	if s.snapshot(choices).Members[0].Ready {
		t.Fatal("contradictory pet object kept authorization")
	}
	if len(s.bindings) != 1 {
		t.Fatal("transient state erased persistent binding")
	}
	obsIdentity(s, "198.51.100.1", "1", true)
	obsAppear(s, "4", "常用")
	if s.snapshot(choices).Members[0].Ready {
		t.Fatal("name-only return authorized")
	}
}
