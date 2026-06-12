package i18n

import (
	"strings"
	"testing"
)

func TestAllKeysHaveBothLanguages(t *testing.T) {
	for key, m := range messages {
		if _, ok := m[EN]; !ok {
			t.Errorf("%s: en 누락", key)
		}
		if _, ok := m[KO]; !ok {
			t.Errorf("%s: ko 누락", key)
		}
	}
}

func TestFmtVerbCountMatches(t *testing.T) {
	for key, m := range messages {
		if strings.Count(m[EN], "%") != strings.Count(m[KO], "%") {
			t.Errorf("%s: fmt verb 개수 불일치 (en=%q, ko=%q)", key, m[EN], m[KO])
		}
	}
}

func TestTFallback(t *testing.T) {
	SetLang(KO)
	defer SetLang(EN)
	if got := T("menu.quit"); got != "종료" {
		t.Fatalf("T(menu.quit) = %q", got)
	}
	if got := T("no.such.key"); got != "no.such.key" {
		t.Fatalf("미존재 키는 키 반환: %q", got)
	}
}

func TestSetLangInvalid(t *testing.T) {
	SetLang(Lang("fr"))
	if Current() != EN {
		t.Fatal("미지원 언어는 EN")
	}
}
