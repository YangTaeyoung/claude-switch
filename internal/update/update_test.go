package update

import (
	"path/filepath"
	"runtime"
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

func TestAssetSelection(t *testing.T) {
	rel := &Release{Assets: []Asset{
		{Name: "claude-switch_1.2.0_darwin_all.tar.gz", URL: "darwin"},
		{Name: "claude-switch_1.2.0_linux_amd64.tar.gz", URL: "linux-amd64"},
		{Name: "claude-switch_1.2.0_linux_arm64.tar.gz", URL: "linux-arm64"},
		{Name: "claude-switch_1.2.0_windows_amd64.zip", URL: "windows"},
		{Name: "checksums.txt", URL: "checksums"},
	}}

	a, ok := rel.asset()
	if !ok {
		t.Fatalf("asset() found nothing for %s", runtime.GOOS)
	}
	switch runtime.GOOS {
	case "darwin":
		if a.URL != "darwin" {
			t.Errorf("darwin: got %q, want universal asset", a.URL)
		}
	case "windows":
		if a.URL != "windows" {
			t.Errorf("windows: got %q, want zip asset", a.URL)
		}
	case "linux":
		// linux는 GOARCH에 따라 amd64/arm64 중 하나를 고른다.
		if a.URL != "linux-amd64" && a.URL != "linux-arm64" {
			t.Errorf("linux: got %q, want a linux tar.gz", a.URL)
		}
	}

	// 호환 에셋이 없으면 false.
	empty := &Release{Assets: []Asset{{Name: "checksums.txt", URL: "x"}}}
	if _, ok := empty.asset(); ok {
		t.Error("asset() should return false when no compatible asset exists")
	}
}
