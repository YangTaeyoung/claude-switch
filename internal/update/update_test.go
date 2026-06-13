package update

import (
	"path/filepath"
	"testing"
	"time"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"1.2.0", "1.3.0", true},
		{"1.2.0", "v1.3.0", true},
		{"v1.2.0", "1.2.1", true},
		{"1.2.0", "2.0.0", true},
		{"1.2.0", "1.2.0", false},
		{"1.3.0", "1.2.0", false},
		{"2.0.0", "1.9.9", false},
		{"1.2.0", "1.2.0-rc1", false}, // 동일 버전의 프리릴리스는 더 새 것으로 보지 않음
		{"dev", "1.0.0", false},       // 로컬 빌드는 항상 false
		{"", "1.0.0", false},
		{"1.2.0", "garbage", false},
		{"1.2", "1.3", true}, // 두 자리 버전도 허용
	}
	for _, c := range cases {
		if got := IsNewer(c.current, c.latest); got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestParseSemver(t *testing.T) {
	if v, ok := parseSemver("v1.2.3"); !ok || v != [3]int{1, 2, 3} {
		t.Errorf("parseSemver(v1.2.3) = %v, %v", v, ok)
	}
	if _, ok := parseSemver("1.2.3.4"); ok {
		t.Error("parseSemver should reject 4-part version")
	}
	if _, ok := parseSemver("abc"); ok {
		t.Error("parseSemver should reject non-numeric")
	}
}

func TestShouldCheck(t *testing.T) {
	dir := t.TempDir()
	stamp := filepath.Join(dir, ".last-update-check")

	// 스탬프 없음 → 확인 필요
	if !ShouldCheck(stamp) {
		t.Error("ShouldCheck should be true when stamp is missing")
	}

	// 방금 확인 → 건너뜀
	MarkChecked(stamp, time.Now())
	if ShouldCheck(stamp) {
		t.Error("ShouldCheck should be false right after MarkChecked")
	}

	// 25시간 전 확인 → 다시 확인 필요
	MarkChecked(stamp, time.Now().Add(-25*time.Hour))
	if !ShouldCheck(stamp) {
		t.Error("ShouldCheck should be true after checkInterval elapsed")
	}
}

func TestTarballSelection(t *testing.T) {
	rel := &Release{Assets: []Asset{
		{Name: "claude-switch_1.2.0_darwin_all.tar.gz", URL: "u1"},
		{Name: "checksums.txt", URL: "u2"},
	}}
	a, ok := rel.tarball()
	if !ok || a.URL != "u1" {
		t.Errorf("tarball() = %+v, %v; want darwin_all asset", a, ok)
	}
}
