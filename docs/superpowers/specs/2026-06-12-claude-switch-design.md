# claude-switch 설계 문서

날짜: 2026-06-12
상태: 승인됨

## 목적

Claude Code 구독 계정을 여러 개 등록해두고, 토큰 리밋이 차면 CLI 명령 하나로 다음 계정으로 전환하는 macOS 전용 Go CLI 도구.

## 동작 원리

macOS에서 Claude Code는 OAuth 자격증명(accessToken/refreshToken/expiresAt 등 JSON)을 키체인 항목 `Claude Code-credentials`에 저장한다
(근거: [공식 인증 문서](https://code.claude.com/docs/en/authentication) — "On macOS, credentials are stored in the encrypted macOS Keychain").
이 항목을 프로필 단위로 스냅샷·교체하여 계정을 전환한다.

`CLAUDE_CONFIG_DIR`은 Linux/Windows에서만 `.credentials.json` 경로를 바꾸므로 macOS에서는 프로필 디렉토리 분리로 계정 격리가 불가능하다. 따라서 키체인 스왑 방식을 채택한다.

### 핵심: sync-back (토큰 회전 대응)

Claude Code는 사용 중 refresh token을 회전시킨다. 등록 시점 스냅샷을 그대로 두면 복원 시점에 무효화된 토큰일 수 있다.
따라서 `use`/`next`는 **전환 전에 현재 키체인 내용을 현재 활성 프로필에 먼저 동기화(sync-back)** 한 뒤 교체한다.

## 명령어

| 명령 | 동작 |
|------|------|
| `claude-switch save <name>` | 현재 키체인 자격증명 + `~/.claude.json`의 `oauthAccount`를 프로필로 저장. 기존 이름이면 덮어쓰기. 활성 프로필로 표시 |
| `claude-switch use <name>` | sync-back → 지정 프로필을 키체인과 `oauthAccount`에 기록 |
| `claude-switch next` | sync-back → 등록 순서상 다음 프로필로 순환 전환 |
| `claude-switch list` | 프로필 목록 + 활성 표시 + 계정 이메일 |
| `claude-switch status` | 활성 프로필 + 계정별 리밋 상태(베스트 에포트) |
| `claude-switch delete <name>` | 프로필 삭제 (키체인 항목 + 메타데이터). 활성 프로필은 삭제 불가 |

## 저장 구조

비밀값은 전부 키체인에만 저장한다. 평문 파일에 토큰을 절대 쓰지 않는다.

- **프로필 자격증명**: 키체인 generic password — `service=claude-switch-profile`, `account=<프로필명>`, 값 = Claude Code 키체인 항목의 JSON 원문
- **메타데이터** (`~/.config/claude-switch/config.json`): 프로필 순서, 활성 프로필명, 프로필별 계정 이메일(표시용). 비밀값 없음
- 키체인 접근은 `/usr/bin/security` CLI 서브프로세스 호출 (cgo 의존성 없음)
  - 읽기: `security find-generic-password -s <service> -a <account> -w`
  - 쓰기: `security add-generic-password -U -s <service> -a <account> -w <json>`
  - 삭제: `security delete-generic-password -s <service> -a <account>`
- 첫 접근 시 macOS 키체인 허용 프롬프트가 뜰 수 있음 (README에 안내)

## 전환 시 함께 처리하는 것

1. `~/.claude.json`의 `oauthAccount` 필드를 프로필의 것으로 교체 — 다른 필드는 보존하는 read-modify-write (`/status` 계정 표시 일치 목적)
2. 전환 후 안내 출력: "새 세션부터 적용됩니다. 실행 중인 claude 세션은 재시작하세요."

## 리밋 상태 표시 (status, 베스트 에포트)

각 프로필의 accessToken으로 가벼운 인증 요청을 보내 `anthropic-ratelimit-unified-*` 응답 헤더(상태/리셋 시각)를 읽는 방식을 시도한다.

- **구현 시점 검증 필요**: 어떤 엔드포인트가 OAuth 토큰으로 저비용 응답 + 리밋 헤더를 주는지 실측으로 확인
- 검증 실패/네트워크 오류 시 해당 계정은 "확인 불가"로 표시. 다른 기능에 영향 없도록 격리

## 구현 스택

- Go 1.25, stdlib 중심 (서브커맨드 6개 — cobra 등 외부 의존성 없이 단순 디스패치)
- 위치: `~/dev/personal/claude-switch`
- 키체인(`security` 실행)과 파일시스템을 인터페이스 뒤로 분리 → 프로필 순환·sync-back·JSON 병합 로직은 단위 테스트
- macOS 전용. Linux/Windows `.credentials.json` 지원은 범위 밖

## 에러 처리

- 키체인 항목 없음(로그인 안 된 상태에서 save): "claude /login으로 먼저 로그인하세요" 안내
- 프로필 0~1개에서 next: 전환할 대상 없음 안내
- `~/.claude.json` 파싱 실패: oauthAccount 교체만 건너뛰고 경고 출력 (키체인 교체는 수행)
- security 명령 실패: stderr 포함해 그대로 노출

## 사용 흐름

```
# 계정 등록 (계정마다 1회)
claude /login            # A계정 로그인
claude-switch save work
claude /login            # B계정 로그인
claude-switch save personal

# 리밋 차면
claude-switch next       # 다음 계정으로 순환
claude-switch use work   # 또는 이름 지정
claude-switch status     # 현재 활성 계정 + 리밋 상태
```
