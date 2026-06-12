package tui

import "testing"

func TestBar(t *testing.T) {
	if got := bar(0, 10); got != "░░░░░░░░░░" {
		t.Fatalf("bar(0) = %q", got)
	}
	if got := bar(1, 10); got != "██████████" {
		t.Fatalf("bar(1) = %q", got)
	}
	if got := bar(0.5, 10); got != "█████░░░░░" {
		t.Fatalf("bar(0.5) = %q", got)
	}
	if got := bar(1.7, 10); got != "██████████" {
		t.Fatalf("bar(>1) 클램프 실패: %q", got)
	}
}

func TestUtilColor(t *testing.T) {
	if utilColor(0.49) != colorOK {
		t.Fatal("0.49는 초록")
	}
	if utilColor(0.5) != colorWarn {
		t.Fatal("0.5는 노랑")
	}
	if utilColor(0.79) != colorWarn {
		t.Fatal("0.79는 노랑")
	}
	if utilColor(0.8) != colorDanger {
		t.Fatal("0.8은 빨강")
	}
}
