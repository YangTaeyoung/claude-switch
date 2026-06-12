# claude-switch

Claude Code 구독 계정을 여러 개 등록해두고 명령 하나로 전환하는 macOS 전용 CLI.
토큰 리밋이 차면 `claude-switch next` 한 번으로 다음 계정으로 넘어간다.

> ⚠️ 이 저장소에는 어떤 시크릿·계정 정보도 커밋하지 않는다. 자격증명은 macOS 키체인에만 저장된다.

## 설치

```shell
go install github.com/YangTaeyoung/claude-switch@latest
# 또는 로컬 빌드
go build -o bin/claude-switch . && cp bin/claude-switch /usr/local/bin/
```

## 사용법

```shell
# 계정 등록 (계정마다 1회)
claude            # /login 으로 A계정 로그인 후 종료
claude-switch save work
claude            # /login 으로 B계정 로그인 후 종료
claude-switch save personal

# 리밋이 차면
claude-switch next            # 다음 계정으로 순환 전환
claude-switch use work        # 또는 이름 지정 전환

# 상태 확인
claude-switch list            # 프로필 목록 (* = 활성)
claude-switch status          # 계정별 리밋 사용률(5h/7d)과 리셋 시각
claude-switch delete <name>   # 프로필 삭제 (활성 프로필은 불가)
```

`status` 출력 예시:

```
활성 프로필: work

* work      work@example.com      리밋: allowed | 5h 80% (리셋 06-12 16:00) | 7d 12% (리셋 06-19 14:00)
  personal  personal@example.com  리밋: allowed | 5h 3% (리셋 06-12 18:00) | 7d 40% (리셋 06-15 09:00)
```

## 동작 원리

macOS에서 Claude Code는 OAuth 자격증명을 키체인 항목 `Claude Code-credentials`에 저장한다
([공식 문서](https://code.claude.com/docs/en/authentication)). claude-switch는:

1. **save**: 현재 키체인 자격증명을 프로필별 키체인 항목(`claude-switch-profile`)으로 스냅샷
2. **use/next**: 전환 전에 현재 키체인 내용을 활성 프로필에 **sync-back**(Claude Code의 refresh token 회전 대응) → 대상 프로필의 자격증명을 `Claude Code-credentials`에 기록 → `~/.claude.json`의 `oauthAccount`도 함께 교체
3. 프로필 순서·활성 프로필 같은 메타데이터만 `~/.config/claude-switch/config.json`에 저장 (토큰 없음)

## 한계 및 주의사항

- **실행 중인 claude 세션에는 적용되지 않는다.** 전환 후 새 세션부터 적용되므로 기존 세션은 재시작할 것.
- **리밋 조회는 최소 inference 요청을 사용한다.** `status`는 프로필당 haiku 1토큰짜리 요청을 보내 `anthropic-ratelimit-unified-*` 헤더를 읽는다 (무과금 엔드포인트들은 리밋 헤더를 주지 않는 것을 실측으로 확인). 사용량에 극미량 가산된다.
- 첫 키체인 접근 시 macOS 허용 프롬프트가 뜰 수 있다 ("항상 허용" 선택 가능).
- `security add-generic-password -w`의 특성상 쓰기 순간 비밀값이 프로세스 인자에 잠깐 노출된다. 로컬 단일 사용자 머신 전제.
- 디버깅: `CLAUDE_SWITCH_DEBUG=1 claude-switch status`로 리밋 응답 헤더를 확인할 수 있다.
