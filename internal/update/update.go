// Package update는 GitHub Releases를 기준으로 새 버전을 확인하고,
// 선택적으로 현재 실행 바이너리를 자가 교체(self-update)한다.
// 모든 네트워크 동작은 베스트 에포트다 — 실패는 호출부에서 조용히 무시한다.
package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/minio/selfupdate"
)

// repo는 릴리스를 조회할 GitHub 저장소다.
const repo = "YangTaeyoung/claude-switch"

// checkInterval은 자동 버전 확인 최소 간격이다 (TUI 시작마다 호출되므로 캐시한다).
const checkInterval = 24 * time.Hour

// Release는 GitHub 릴리스 하나의 관심 필드다.
type Release struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

// Asset은 릴리스에 첨부된 파일이다.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Latest는 GitHub Releases API로 최신 릴리스를 조회한다.
func Latest(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github releases API returned HTTP %d", resp.StatusCode)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	if rel.TagName == "" {
		return nil, fmt.Errorf("no tag_name in latest release")
	}
	return &rel, nil
}

// IsNewer는 latest가 current보다 높은 버전인지 판단한다.
// current가 "dev"(로컬 빌드)이거나 어느 한쪽이 파싱 불가하면 false다.
func IsNewer(current, latest string) bool {
	if current == "" || current == "dev" {
		return false
	}
	c, ok1 := parseSemver(current)
	l, ok2 := parseSemver(latest)
	if !ok1 || !ok2 {
		return false
	}
	for i := 0; i < 3; i++ {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// parseSemver는 "v1.2.3" / "1.2.3" 형식을 [3]int로 파싱한다.
// 프리릴리스/빌드 메타데이터(-, +)는 무시한다.
func parseSemver(v string) ([3]int, bool) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// tarball은 현재 OS에 맞는 tar.gz 에셋을 찾는다.
// GoReleaser 유니버설 바이너리(darwin_all)와 일반 에셋을 모두 처리한다.
func (r *Release) tarball() (Asset, bool) {
	// 유니버설 바이너리(darwin_all) 우선.
	for _, a := range r.Assets {
		if strings.HasSuffix(a.Name, "_"+runtime.GOOS+"_all.tar.gz") {
			return a, true
		}
	}
	for _, a := range r.Assets {
		if strings.Contains(a.Name, runtime.GOOS) && strings.HasSuffix(a.Name, ".tar.gz") {
			return a, true
		}
	}
	return Asset{}, false
}

// SelfUpdate는 릴리스의 tar.gz 에셋을 내려받아 현재 실행 바이너리를 교체한다.
// 교체 실패 시 selfupdate가 이전 바이너리로 롤백을 시도한다.
func SelfUpdate(ctx context.Context, rel *Release) error {
	asset, ok := rel.tarball()
	if !ok {
		return fmt.Errorf("no compatible release asset for %s", runtime.GOOS)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	bin, err := extractBinary(resp.Body, "claude-switch")
	if err != nil {
		return err
	}

	if err := selfupdate.Apply(bin, selfupdate.Options{}); err != nil {
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			return fmt.Errorf("update failed and rollback failed: %w", rerr)
		}
		return fmt.Errorf("update failed (rolled back): %w", err)
	}
	return nil
}

// extractBinary는 gzip+tar 스트림에서 지정 이름의 파일을 찾아 Reader로 돌려준다.
func extractBinary(r io.Reader, name string) (io.Reader, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("binary %q not found in archive", name)
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == name {
			return tr, nil
		}
	}
}

// ShouldCheck는 마지막 확인으로부터 checkInterval이 지났는지 반환한다.
// 스탬프 파일이 없거나 읽을 수 없으면 true(확인 필요)다.
func ShouldCheck(stampPath string) bool {
	data, err := os.ReadFile(stampPath)
	if err != nil {
		return true
	}
	sec, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return true
	}
	return time.Since(time.Unix(sec, 0)) >= checkInterval
}

// MarkChecked는 현재 시각을 스탬프 파일에 기록한다. 실패는 무시한다.
func MarkChecked(stampPath string, now time.Time) {
	_ = os.MkdirAll(filepath.Dir(stampPath), 0o700)
	_ = os.WriteFile(stampPath, []byte(strconv.FormatInt(now.Unix(), 10)), 0o600)
}

// StampPath는 config 디렉토리 기준 확인 스탬프 파일 경로를 반환한다.
func StampPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), ".last-update-check")
}
