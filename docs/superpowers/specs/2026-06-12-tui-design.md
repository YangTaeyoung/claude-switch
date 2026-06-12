# claude-switch TUI 설계 문서

날짜: 2026-06-12
상태: 승인됨
대상 버전: v1.1.0

## 목적

정적 텍스트 출력뿐인 CLI에 모던 인터랙티브 TUI를 추가한다. 인자 없이 `claude-switch`를 실행하면
프로필 목록·계정별 사용량 게이지를 보여주는 풀스크린 대시보드가 뜨고, 그 자리에서 전환·삭제·새로고침할 수 있다.

## 스택

- `charm.land/bubbletea/v2` — 2026-02-23 정식 릴리즈된 최신 메이저
  (근거: https://github.com/charmbracelet/bubbletea/releases/tag/v2.0.0)
- `charm.land/lipgloss/v2` — 스타일링
- 게이지 바·스피너는 직접 렌더링 (bubbles 의존 없음)

v2 핵심 API (근거: https://github.com/charmbracelet/bubbletea/discussions/1374):
- `View() tea.View` — `tea.NewView(content)` 후 `v.AltScreen = true` 식 선언형 설정
- 키 입력은 `tea.KeyPressMsg`, `msg.String()`으로 "up"/"enter"/"q" 등 비교
- `tea.Cmd`/`tea.Batch` 비동기 패턴은 v1과 동일

## 동작

### 진입
- `claude-switch` (인자 없음) → TUI. 기존엔 usage 출력이었음 → usage는 `claude-switch help`로 이동
- 기존 서브커맨드(save/use/next/list/status/delete)는 전부 그대로 유지 (스크립트용)
- 등록된 프로필이 0개면 TUI 대신 안내 메시지 출력 후 종료

### 화면
```
  claude-switch

  ▸ ● work      work@example.com
      5h  ████████████░░░░░░░░  80%  resets 16:00
      7d  ███░░░░░░░░░░░░░░░░░  12%  resets 06-19
    ○ personal  personal@example.com
      5h  ⠋ loading...

  ✓ Switched to work
  ↑/↓ move · enter switch · r refresh · d delete · q quit
```
- 커서 `▸`, 활성 프로필 `●`(강조색)/비활성 `○`
- 게이지 색: 사용률 <50% 초록, <80% 노랑, ≥80% 빨강
- 사용량은 진입 즉시 프로필별 병렬 fetch, 도착 전 스피너 프레임 표시. 실패 시 "limit unavailable (...)" 회색 표시
- 하단 상태줄: 최근 동작 결과(전환/삭제/에러), 그 아래 키맵 도움말

### 키맵
| 키 | 동작 |
|---|---|
| `↑`/`k`, `↓`/`j` | 커서 이동 |
| `enter` | 커서 프로필로 전환 (app.Use 재사용 — sync-back 포함). 이미 활성이면 상태줄 안내 |
| `r` | 전체 사용량 새로고침 |
| `d` | 삭제 확인 모드 진입 → `y` 확정 / `n`·`esc` 취소. 활성 프로필이면 상태줄에 거부 사유 |
| `q`, `ctrl+c` | 종료 |

## 구조

- 신규 패키지 `internal/tui`
  - `tui.go` — Model(상태)·Update(메시지 처리). 키 처리는 `handleKey(string)` 헬퍼로 분리해 테스트 가능하게
  - `view.go` — View 렌더링, `bar(utilization float64, width int) string`, 색상 선택
- `internal/app` 리팩토링 (동작 변화 없음)
  - `Usage(ctx, cfg, name, check) limit.Result` export — limitLine 내부(프로필별 자격증명 조회→토큰 추출→check 호출) 추출. limitLine은 Usage 결과를 문자열로만 변환
  - `Config() (*config.Config, error)` export — config.Load(a.ConfigPath) 래퍼
  - TUI에서 Use/Delete 호출 시 출력 무시를 위해 Out/Errw를 io.Discard로 둔 App 인스턴스 사용
- `main.go` — 인자 없으면 `tui.Run(app)` 실행

## 데모 모드 (GIF 녹화용)

`CLAUDE_SWITCH_DEMO=1`이면 실제 키체인·config 대신 **가짜 프로필 2개 + 가짜 사용량**으로 TUI 구동.
목적: README 데모 GIF에 실제 이메일·계정 정보가 노출되지 않게 함 (저장소 민감 정보 금지 규칙).
전환/삭제도 인메모리로만 동작. 코드는 `internal/tui/demo.go`에 격리.

## 데모 GIF

- `demo.tape` (VHS 스크립트): 데모 모드로 TUI 실행 → 커서 이동 → 전환 → 새로고침 → 종료 녹화
- 출력 `docs/demo.gif` → 영/한 README 최상단 콘솔 블록을 GIF로 교체 (콘솔 블록은 그 아래 유지)
- vhs는 brew 설치, 녹화는 로컬 1회 (CI 아님)

## 에러 처리

- TUI 내 모든 동작 실패는 상태줄에 빨간 텍스트로 표시하고 TUI는 유지
- 터미널이 TTY가 아니면(파이프 등) TUI 대신 help 출력
- README의 "zero dependencies" 문구는 "minimal dependencies (Charm 스택)"로 수정

## 테스트

- `bar()` 게이지 렌더링·색상 경계(0.49/0.5/0.79/0.8) 단위 테스트
- `handleKey` 기반 Update 흐름: 커서 이동 경계, 삭제 확인 플로우(d→y/d→n), 활성 프로필 삭제 거부
- `app.Usage` 추출은 기존 status 테스트로 회귀 확인
- TUI 실 구동은 데모 모드로 수동 확인 + VHS 녹화로 검증
