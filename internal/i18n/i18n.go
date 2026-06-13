// Package i18n은 claude-switch의 사용자 대면 메시지를 다국어(en/ko)로 제공한다.
// 에러 메시지는 번역하지 않는다 (스크립트·이슈 리포트 안정성).
package i18n

// Lang은 지원 언어 코드다.
type Lang string

const (
	EN Lang = "en"
	KO Lang = "ko"
)

var current = EN

// SetLang은 현재 언어를 설정한다. 미지원 코드는 영어로 둔다.
func SetLang(l Lang) {
	if l == KO {
		current = KO
		return
	}
	current = EN
}

// Current는 현재 언어를 반환한다.
func Current() Lang { return current }

// Valid는 지원하는 언어 코드인지 검사한다.
func Valid(l Lang) bool { return l == EN || l == KO }

// T는 키에 해당하는 현재 언어 메시지를 반환한다. 없으면 영어, 그것도 없으면 키 그대로.
func T(key string) string {
	m, ok := messages[key]
	if !ok {
		return key
	}
	if s, ok := m[current]; ok {
		return s
	}
	return m[EN]
}

var messages = map[string]map[Lang]string{
	// ── TUI: 홈 ──
	"menu.manage":    {EN: "Manage profiles (%d)", KO: "프로필 관리 (%d)"},
	"menu.save":      {EN: "Save current account as profile", KO: "현재 계정을 프로필로 저장"},
	"menu.settings":  {EN: "Settings", KO: "설정"},
	"menu.quit":      {EN: "Quit", KO: "종료"},
	"home.noProfile": {EN: "No profiles yet — save the current account to get started.", KO: "아직 프로필이 없습니다 — 현재 로그인된 계정을 저장해 시작하세요."},
	"home.help":      {EN: "↑/↓ move · enter select · q quit", KO: "↑/↓ 이동 · enter 선택 · q 종료"},

	// ── TUI: 프로필 화면 ──
	"profiles.title":       {EN: "Profiles", KO: "프로필"},
	"profiles.help":        {EN: "↑/↓ move · enter switch · n rename · r refresh · d delete · esc back · q quit", KO: "↑/↓ 이동 · enter 전환 · n 이름변경 · r 새로고침 · d 삭제 · esc 뒤로 · q 종료"},
	"profiles.loading":     {EN: "loading...", KO: "불러오는 중..."},
	"profiles.resets":      {EN: "resets", KO: "리셋"},
	"profiles.noLimit":     {EN: "limit unavailable", KO: "리밋 확인 불가"},
	"confirm.delete":       {EN: "Delete %q? (y/n)", KO: "%q 프로필을 삭제할까요? (y/n)"},
	"status.saved":         {EN: "✓ Saved %s", KO: "✓ %s 프로필을 저장했습니다"},
	"status.switched":      {EN: "✓ Switched to %s", KO: "✓ %s 프로필로 전환했습니다"},
	"status.deleted":       {EN: "✓ Deleted %s", KO: "✓ %s 프로필을 삭제했습니다"},
	"status.renamed":       {EN: "✓ Renamed %s → %s", KO: "✓ 이름 변경: %s → %s"},
	"status.activeDelete":  {EN: "cannot delete the active profile", KO: "활성 프로필은 삭제할 수 없습니다"},
	"status.langSet":       {EN: "✓ Language updated", KO: "✓ 언어가 변경되었습니다"},
	"status.autoUpdateOn":  {EN: "✓ Auto-update enabled", KO: "✓ 자동 업데이트를 켰습니다"},
	"status.autoUpdateOff": {EN: "✓ Auto-update disabled", KO: "✓ 자동 업데이트를 껐습니다"},

	// ── 업데이트 (TUI 상태바) ──
	"update.available": {EN: "⚡ New version %s available — run: claude-switch update", KO: "⚡ 새 버전 %s 사용 가능 — claude-switch update 실행"},
	"update.updating":  {EN: "Downloading update %s...", KO: "업데이트 %s 다운로드 중..."},
	"update.done":      {EN: "✓ Updated to %s — restart to apply", KO: "✓ %s(으)로 업데이트 완료 — 재시작하면 적용됩니다"},

	// ── TUI: 저장/이름변경 입력 ──
	"save.title":    {EN: "Save current account as profile", KO: "현재 계정을 프로필로 저장"},
	"save.prompt":   {EN: "Profile name", KO: "프로필 이름"},
	"save.empty":    {EN: "name cannot be empty", KO: "이름을 입력하세요"},
	"input.help":    {EN: "enter confirm · esc cancel", KO: "enter 확정 · esc 취소"},
	"rename.title":  {EN: "Rename profile %q", KO: "%q 프로필 이름 변경"},
	"rename.prompt": {EN: "New name", KO: "새 이름"},

	// ── TUI: 설정 ──
	"settings.title":      {EN: "Settings", KO: "설정"},
	"settings.language":   {EN: "Language", KO: "언어"},
	"settings.autoUpdate": {EN: "Auto-update on launch", KO: "시작 시 자동 업데이트"},
	"settings.on":         {EN: "on", KO: "켜짐"},
	"settings.off":        {EN: "off", KO: "꺼짐"},
	"settings.help":       {EN: "↑/↓ move · enter apply/toggle · esc back", KO: "↑/↓ 이동 · enter 적용/전환 · esc 뒤로"},

	// ── CLI ──
	"cli.saved":           {EN: "Saved profile %q (active: %s)\n", KO: "프로필 %q 저장 완료 (활성: %s)\n"},
	"cli.switched":        {EN: "Switched to profile %q (%s)\n", KO: "%q 프로필로 전환했습니다 (%s)\n"},
	"cli.takesEffect":     {EN: "Takes effect for new claude sessions. Restart any running session.", KO: "새로 시작하는 claude 세션부터 적용됩니다. 실행 중인 세션은 재시작하세요."},
	"cli.alreadyActive":   {EN: "Profile %q is already active\n", KO: "이미 %q 프로필이 활성입니다\n"},
	"cli.deleted":         {EN: "Deleted profile %q\n", KO: "프로필 %q 삭제 완료\n"},
	"cli.renamed":         {EN: "Renamed profile %q to %q\n", KO: "프로필 이름을 %q에서 %q(으)로 변경했습니다\n"},
	"cli.noProfiles":      {EN: "No profiles registered. Log in with claude /login, then run: claude-switch save <name>", KO: "등록된 프로필이 없습니다. claude /login 후 claude-switch save <name>으로 등록하세요"},
	"cli.activeProfile":   {EN: "Active profile: %s\n\n", KO: "활성 프로필: %s\n\n"},
	"cli.unknownEmail":    {EN: "unknown email", KO: "이메일 미상"},
	"cli.langSet":         {EN: "Language set to %s\n", KO: "언어가 %s(으)로 설정되었습니다\n"},
	"cli.update.dev":      {EN: "Running a dev build — skipping update.\n", KO: "개발 빌드입니다 — 업데이트를 건너뜁니다.\n"},
	"cli.update.checking": {EN: "Checking for updates...\n", KO: "업데이트 확인 중...\n"},
	"cli.update.latest":   {EN: "Already up to date (%s)\n", KO: "이미 최신 버전입니다 (%s)\n"},
	"cli.update.found":    {EN: "Updating %s → %s ...\n", KO: "업데이트 중: %s → %s ...\n"},
	"cli.update.done":     {EN: "✓ Updated to %s. Restart claude-switch to use it.\n", KO: "✓ %s(으)로 업데이트 완료. claude-switch를 재시작하면 적용됩니다.\n"},
	"cli.usage": {
		EN: `claude-switch — switch between Claude Code subscription accounts

Usage:
  claude-switch                Interactive dashboard (TUI)
  claude-switch save <name>    Save the currently logged-in account as a profile
  claude-switch use <name>     Switch to a specific profile
  claude-switch next           Cycle to the next profile
  claude-switch list           List profiles (* = active)
  claude-switch status         Per-account usage limits and reset times
  claude-switch rename <old> <new>  Rename a profile
  claude-switch delete <name>  Delete a profile
  claude-switch lang <en|ko>   Set display language
  claude-switch update         Update to the latest release
  claude-switch version        Print version

Register each account once:
  run claude → log in with /login → claude-switch save <name>
`,
		KO: `claude-switch — Claude Code 구독 계정 전환 도구

사용법:
  claude-switch                인터랙티브 대시보드 (TUI)
  claude-switch save <name>    현재 로그인된 계정을 프로필로 저장
  claude-switch use <name>     지정 프로필로 전환
  claude-switch next           다음 프로필로 순환 전환
  claude-switch list           프로필 목록 (* = 활성)
  claude-switch status         계정별 사용량과 리셋 시각
  claude-switch rename <old> <new>  프로필 이름 변경
  claude-switch delete <name>  프로필 삭제
  claude-switch lang <en|ko>   표시 언어 설정
  claude-switch update         최신 릴리스로 업데이트
  claude-switch version        버전 출력

계정 등록 (계정마다 1회):
  claude 실행 → /login 으로 로그인 → claude-switch save <name>
`,
	},
}
