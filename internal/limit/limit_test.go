package limit

import (
	"net/http"
	"testing"
	"time"
)

func TestParseHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("anthropic-ratelimit-unified-status", "allowed")
	h.Set("anthropic-ratelimit-unified-5h-status", "allowed")
	h.Set("anthropic-ratelimit-unified-5h-utilization", "0.77")
	h.Set("anthropic-ratelimit-unified-5h-reset", "1781247600")
	h.Set("anthropic-ratelimit-unified-7d-status", "allowed")
	h.Set("anthropic-ratelimit-unified-7d-utilization", "0.11")

	r := parseHeaders(h)
	if r.Status != "allowed" {
		t.Fatalf("Status = %q", r.Status)
	}
	if r.FiveHour.Utilization != 0.77 {
		t.Fatalf("5h Utilization = %v", r.FiveHour.Utilization)
	}
	if !r.FiveHour.ResetsAt.Equal(time.Unix(1781247600, 0)) {
		t.Fatalf("5h ResetsAt = %v", r.FiveHour.ResetsAt)
	}
	if r.SevenDay.Utilization != 0.11 {
		t.Fatalf("7d Utilization = %v", r.SevenDay.Utilization)
	}
	if !r.SevenDay.ResetsAt.IsZero() {
		t.Fatalf("7d ResetsAt = %v, want zero", r.SevenDay.ResetsAt)
	}
}

func TestParseHeadersEmpty(t *testing.T) {
	r := parseHeaders(http.Header{})
	if r.Status != "" {
		t.Fatalf("Status = %q, want empty", r.Status)
	}
	if r.FiveHour.Utilization != -1 {
		t.Fatalf("5h Utilization = %v, want -1", r.FiveHour.Utilization)
	}
}

func TestAccessToken(t *testing.T) {
	t.Run("claudeAiOauth 하위", func(t *testing.T) {
		got, err := AccessToken(`{"claudeAiOauth":{"accessToken":"fake-token-x"}}`)
		if err != nil {
			t.Fatal(err)
		}
		if got != "fake-token-x" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("최상위", func(t *testing.T) {
		got, err := AccessToken(`{"accessToken":"fake-token-y"}`)
		if err != nil {
			t.Fatal(err)
		}
		if got != "fake-token-y" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("없으면 에러", func(t *testing.T) {
		if _, err := AccessToken(`{}`); err == nil {
			t.Fatal("에러를 기대")
		}
	})
}
