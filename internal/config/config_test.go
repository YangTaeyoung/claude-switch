package config

import (
	"path/filepath"
	"testing"
)

func TestNext(t *testing.T) {
	t.Run("프로필이 2개 미만이면 에러", func(t *testing.T) {
		c := &Config{Profiles: []Profile{{Name: "only"}}, Active: "only"}
		if _, err := c.Next(); err == nil {
			t.Fatal("에러를 기대했으나 nil")
		}
	})
	t.Run("활성 프로필 다음 순서로 순환", func(t *testing.T) {
		c := &Config{
			Profiles: []Profile{{Name: "a"}, {Name: "b"}, {Name: "c"}},
			Active:   "c",
		}
		got, err := c.Next()
		if err != nil {
			t.Fatal(err)
		}
		if got != "a" {
			t.Fatalf("got %q, want %q", got, "a")
		}
	})
	t.Run("활성 프로필이 목록에 없으면 첫 프로필", func(t *testing.T) {
		c := &Config{Profiles: []Profile{{Name: "a"}, {Name: "b"}}, Active: "deleted"}
		got, err := c.Next()
		if err != nil {
			t.Fatal(err)
		}
		if got != "a" {
			t.Fatalf("got %q, want %q", got, "a")
		}
	})
}

func TestUpsert(t *testing.T) {
	c := &Config{}
	c.Upsert(Profile{Name: "work", Email: "old"})
	c.Upsert(Profile{Name: "work", Email: "new"})
	if len(c.Profiles) != 1 {
		t.Fatalf("프로필 수 = %d, want 1", len(c.Profiles))
	}
	if c.Profiles[0].Email != "new" {
		t.Fatalf("Email = %q, want %q", c.Profiles[0].Email, "new")
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")

	missing, err := Load(path)
	if err != nil {
		t.Fatalf("없는 파일 Load 실패: %v", err)
	}
	if len(missing.Profiles) != 0 {
		t.Fatal("없는 파일은 빈 Config여야 함")
	}

	c := &Config{Active: "work", Profiles: []Profile{{Name: "work", Email: "fake@example.com"}}}
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Active != "work" || len(loaded.Profiles) != 1 || loaded.Profiles[0].Name != "work" {
		t.Fatalf("round-trip 불일치: %+v", loaded)
	}
}

func TestRemove(t *testing.T) {
	c := &Config{Profiles: []Profile{{Name: "a"}, {Name: "b"}}}
	if !c.Remove("a") {
		t.Fatal("Remove(a) = false, want true")
	}
	if c.Find("a") != nil {
		t.Fatal("a가 남아 있음")
	}
	if c.Remove("ghost") {
		t.Fatal("Remove(ghost) = true, want false")
	}
}
