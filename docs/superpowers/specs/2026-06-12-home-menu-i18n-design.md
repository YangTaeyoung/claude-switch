# claude-switch 홈 메뉴·다국어·rename 설계 문서

날짜: 2026-06-12
상태: 승인됨
대상 버전: v1.2.0

## 목적

1. TUI 진입 시 ASCII 로고+버전이 있는 **홈 메뉴**에서 작업을 선택하도록 개편
2. 프로필 0개일 때 저장 유도 플로우
3. 프로필 **이름 변경**(TUI `n` 키 + CLI `rename <old> <new>`)
4. **다국어**(en/ko) — config `language` 설정, TUI Settings와 `lang ko|en` CLI로 전환
5. 데모 GIF 비율 수정 — 녹화 후 프레임을 직접 확인해 콘텐츠에 맞는 크기로 재설정

## TUI 상태 머신

```
home ──enter──▸ profiles (Manage)   profiles ──n──▸ renameInput ──enter/esc──▸ profiles
  │──enter──▸ saveInput ──enter──▸ profiles        profiles ──esc──▸ home
  │──enter──▸ settings ──esc──▸ home               saveInput ──esc──▸ home
  └──enter(Quit)/q──▸ 종료
```

- home 메뉴: Manage profiles (N) / Save current account as profile / Settings / Quit
- 프로필 0개: 로고 아래 안내 문구 + 커서 기본 위치 = Save. Manage 선택 시 안내만 표시
- saveInput: 이름 입력(bubbles/v2 textinput, 부적합 시 직접 구현). enter → app.Save → profiles로 이동. 키체인에 자격증명 없으면 에러를 상태줄에 표시
- renameInput: 기존 이름 표시 + 새 이름 입력 → app.Rename
- settings: Language 항목 — English/한국어 토글. 선택 즉시 config 저장 + i18n 전환 + 화면 반영
- 기존 profiles 화면의 키(이동/enter/r/d/q)는 유지, `q`는 home이 아닌 화면에선 동작 유지(종료)

## app.Rename(old, new string) error

1. old 존재·new 비어있지 않음·new 미중복 검사
2. 키체인: `Get(ProfileService, old)` → `Set(ProfileService, new, cred)` → `Delete(ProfileService, old)`
3. config: Profile.Name 변경, Active==old면 Active=new, 저장
4. CLI `rename <old> <new>` 서브커맨드 추가

## i18n (internal/i18n)

- `Lang` 타입("en"/"ko"), `SetLang`, `T(key string) string`. 키 누락 시 en 폴백
- config.Config에 `Language string` 필드 (빈 값 = "en")
- 적용: TUI 전체 텍스트, CLI usage, CLI 성공 메시지(Saved/Switched/Deleted/Renamed 등), 안내 메시지
- 에러 메시지는 영어 유지
- `lang ko|en` 서브커맨드: config.Language 저장
- 테스트: en/ko 키셋 일치 검증

## 버전 표기

- main.go `var version = "dev"`, GoReleaser ldflags `-s -w -X main.version={{.Version}}`
- 홈 화면 로고 아래 `v{version}` 표시, `claude-switch version` 서브커맨드

## ASCII 로고

박스드로잉/figlet풍 소형 로고(폭 50 이내), lipgloss 보라 계열 스타일. 데모·실사용 동일.

## 데모 GIF

- 새 플로우: `claude-switch` → 홈(로고·메뉴) → Manage 진입 → 사용량 로딩 → 전환 → 종료
- 데모 모드(demoBackend)가 Save/Rename도 인메모리 지원
- **녹화 후 ffmpeg로 프레임을 추출해 직접 확인**, 빈 여백이 크면 Width/Height를 콘텐츠에 맞게 조정해 재녹화 (이전 문제: 콘텐츠 ~14행에 Height 620 → 하단 절반 공백)

## 테스트

- tui: home 메뉴 이동·선택 → 화면 전환, save 입력 플로우, rename 플로우, settings 언어 전환(config 반영)
- app: Rename 성공/중복/미존재/active 갱신 (fake keychain)
- i18n: en/ko 키셋 동치
- 기존 테스트 회귀 유지
