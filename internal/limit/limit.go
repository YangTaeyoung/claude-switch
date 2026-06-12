// Package limit은 각 계정의 리밋 상태를 베스트 에포트로 조회한다.
// 실패는 정상 경로다 — 호출부는 Err가 있으면 "확인 불가"로 표시한다.
package limit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// probeURL은 리밋 헤더(anthropic-ratelimit-unified-*)를 받기 위한 최소 inference 요청.
// /api/oauth/profile, /v1/messages/count_tokens는 인증은 되지만 리밋 헤더가 없는 것을 실측으로 확인함.
// 프로브 1회당 haiku 1토큰 출력만큼의 사용량이 발생한다.
const probeURL = "https://api.anthropic.com/v1/messages"

const probeBody = `{"model":"claude-haiku-4-5-20251001","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`

// Window는 리밋 윈도우(5시간/7일) 하나의 상태다.
type Window struct {
	// Status는 allowed / allowed_warning / rejected 등.
	Status string
	// Utilization은 사용률 (0~1). 헤더가 없으면 -1.
	Utilization float64
	// ResetsAt은 윈도우 리셋 시각. 헤더가 없으면 zero value.
	ResetsAt time.Time
}

type Result struct {
	// Status는 대표 리밋 상태 (anthropic-ratelimit-unified-status).
	Status   string
	FiveHour Window
	SevenDay Window
	Err      error
}

// Check는 accessToken으로 최소 inference 요청을 보내 리밋 헤더를 읽는다.
func Check(ctx context.Context, accessToken string) Result {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, probeURL, strings.NewReader(probeBody))
	if err != nil {
		return Result{Err: err}
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return Result{Err: err}
	}
	defer resp.Body.Close()

	if os.Getenv("CLAUDE_SWITCH_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[debug] HTTP %d\n", resp.StatusCode)
		for k, v := range resp.Header {
			fmt.Fprintf(os.Stderr, "[debug] %s: %s\n", k, strings.Join(v, ", "))
		}
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return Result{Err: fmt.Errorf("authentication failed (HTTP %d) — token may be expired", resp.StatusCode)}
	}

	r := parseHeaders(resp.Header)
	if r.Status == "" {
		r.Err = fmt.Errorf("no rate-limit headers (HTTP %d)", resp.StatusCode)
	}
	return r
}

// parseHeaders는 anthropic-ratelimit-unified-* 헤더를 Result로 변환한다.
func parseHeaders(h http.Header) Result {
	return Result{
		Status:   h.Get("anthropic-ratelimit-unified-status"),
		FiveHour: parseWindow(h, "5h"),
		SevenDay: parseWindow(h, "7d"),
	}
}

func parseWindow(h http.Header, window string) Window {
	w := Window{Status: h.Get("anthropic-ratelimit-unified-" + window + "-status"), Utilization: -1}
	if util := h.Get("anthropic-ratelimit-unified-" + window + "-utilization"); util != "" {
		if f, err := strconv.ParseFloat(util, 64); err == nil {
			w.Utilization = f
		}
	}
	if reset := h.Get("anthropic-ratelimit-unified-" + window + "-reset"); reset != "" {
		if sec, err := strconv.ParseInt(reset, 10, 64); err == nil {
			w.ResetsAt = time.Unix(sec, 0)
		}
	}
	return w
}

// AccessToken은 키체인 자격증명 JSON에서 accessToken을 추출한다.
// 최상위 또는 claudeAiOauth 하위에서 찾는다.
func AccessToken(credJSON string) (string, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(credJSON), &doc); err != nil {
		return "", fmt.Errorf("failed to parse credentials JSON: %w", err)
	}
	if raw, ok := doc["claudeAiOauth"]; ok {
		if err := json.Unmarshal(raw, &doc); err != nil {
			return "", fmt.Errorf("failed to parse claudeAiOauth: %w", err)
		}
	}
	var token string
	if raw, ok := doc["accessToken"]; ok {
		if err := json.Unmarshal(raw, &token); err != nil {
			return "", err
		}
	}
	if token == "" {
		return "", fmt.Errorf("accessToken field not found")
	}
	return token, nil
}
