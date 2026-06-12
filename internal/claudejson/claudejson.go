// Package claudejson은 ~/.claude.json의 oauthAccount 필드를 다른 필드를 보존한 채 읽고 쓴다.
package claudejson

import (
	"encoding/json"
	"fmt"
	"os"
)

// ReadOAuthAccount는 oauthAccount 필드를 원문 그대로 반환한다. 필드가 없으면 nil.
func ReadOAuthAccount(path string) (json.RawMessage, error) {
	doc, err := load(path)
	if err != nil {
		return nil, err
	}
	return doc["oauthAccount"], nil
}

// WriteOAuthAccount는 다른 필드를 보존한 채 oauthAccount만 교체한다.
func WriteOAuthAccount(path string, oauthAccount json.RawMessage) error {
	doc, err := load(path)
	if err != nil {
		return err
	}
	if len(oauthAccount) == 0 {
		delete(doc, "oauthAccount")
	} else {
		doc["oauthAccount"] = oauthAccount
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o600)
}

// Email은 oauthAccount JSON에서 이메일을 추출한다. 없으면 빈 문자열.
func Email(oauthAccount json.RawMessage) string {
	var m map[string]any
	if json.Unmarshal(oauthAccount, &m) != nil {
		return ""
	}
	for _, key := range []string{"emailAddress", "email"} {
		if s, ok := m[key].(string); ok {
			return s
		}
	}
	return ""
}

func load(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return doc, nil
}
