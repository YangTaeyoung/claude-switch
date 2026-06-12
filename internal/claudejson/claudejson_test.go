package claudejson

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".claude.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadOAuthAccount(t *testing.T) {
	path := writeFixture(t, `{"oauthAccount":{"emailAddress":"fake@example.com"},"other":1}`)
	got, err := ReadOAuthAccount(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"emailAddress":"fake@example.com"}` {
		t.Fatalf("got %s", got)
	}
}

func TestWriteOAuthAccountPreservesOtherFields(t *testing.T) {
	path := writeFixture(t, `{"oauthAccount":{"emailAddress":"old@example.com"},"numStartups":42,"projects":{"/tmp":{"history":[]}}}`)

	if err := WriteOAuthAccount(path, json.RawMessage(`{"emailAddress":"new@example.com"}`)); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if string(doc["oauthAccount"]) != `{"emailAddress":"new@example.com"}` {
		t.Fatalf("oauthAccount = %s", doc["oauthAccount"])
	}
	if string(doc["numStartups"]) != "42" {
		t.Fatalf("numStartups 보존 실패: %s", doc["numStartups"])
	}
	if _, ok := doc["projects"]; !ok {
		t.Fatal("projects 필드 유실")
	}
}

func TestEmail(t *testing.T) {
	if got := Email(json.RawMessage(`{"emailAddress":"a@example.com"}`)); got != "a@example.com" {
		t.Fatalf("got %q", got)
	}
	if got := Email(json.RawMessage(`{"email":"b@example.com"}`)); got != "b@example.com" {
		t.Fatalf("got %q", got)
	}
	if got := Email(json.RawMessage(`not-json`)); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}
