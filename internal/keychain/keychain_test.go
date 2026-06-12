package keychain

import "testing"

const sampleMeta = `keychain: "/Users/fakeuser/Library/Keychains/login.keychain-db"
version: 512
class: "genp"
attributes:
    0x00000007 <blob>="Claude Code-credentials"
    "acct"<blob>="fakeuser"
    "svce"<blob>="Claude Code-credentials"
`

func TestParseAccount(t *testing.T) {
	if got := parseAccount(sampleMeta); got != "fakeuser" {
		t.Fatalf("got %q, want %q", got, "fakeuser")
	}
	if got := parseAccount("no acct line"); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}
