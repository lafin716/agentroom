# agentroom

LLM 코딩 에이전트를 사내 프로젝트에 안전하게 사용하기 위한 CLI 도구. 메인 프로젝트의 "민감 정보 제외 사본"을 만들고, 에이전트는 그 사본에서만 작업하며, 작업이 끝나면 변경분만 메인으로 안전하게 동기화합니다 (충돌 감지 + LIFO undo).

## 설치

```bash
go build -o ar ./apps/cli      # Linux/macOS
go build -o ar.exe ./apps/cli  # Windows
```

## 사용 흐름

```bash
# 1) 메인 프로젝트에서 초기화
cd /path/to/your-project
ar init                        # .agentignore (템플릿) + .agent/config.json 생성

# 2) .agentignore에 민감 정보 경로를 추가
vi .agentignore

# 3) 사본 생성 (기본: ~/.agentroom/workspaces/<name>-<hash>/)
ar copy                        # 또는 ar copy --dest /custom/path

# 4) 에이전트에게 ~/.agentroom/workspaces/<name>-<hash>/ 에서 작업하게 시킴
#    .env, *.key 등은 거기 없으므로 에이전트 컨텍스트에 절대 노출되지 않음

# 5) 작업 후 메인에서 상태 확인
ar status

# 6) 메인으로 동기화 (변경 미리보기 + 확인)
ar sync                        # --yes 로 확인 스킵, --force 로 충돌 무시
                               # --with-delete=false 로 삭제는 적용하지 않음

# 7) 마음에 안 들면 되돌리기 (가장 최근 sync 부터 역순)
ar history
ar undo                        # -n 3 으로 3회 연속 되돌리기
```

## 보안 모델

- `.agentignore` 매칭 항목은:
  - **사본에 절대 복사되지 않음** (에이전트가 볼 수 없음)
  - **sync에서도 절대 건드리지 않음** (메인의 원본 그대로 유지)
- `.agent/` 폴더와 `.git/` 은 항상 무시 (하드코딩)
- 충돌(메인과 사본이 동시에 같은 파일 변경) 시 sync는 기본적으로 중단

## 모노레포 구조

```
agentroom/
├── go.work
├── apps/
│   ├── cli/        # ar 바이너리
│   └── gui/        # (placeholder) 향후 GUI
├── pkg/            # 공유 라이브러리 (CLI/GUI 공용)
│   ├── workspace/
│   ├── ignore/
│   ├── index/
│   ├── copier/
│   ├── differ/
│   ├── snapshot/
│   ├── syncer/
│   └── logx/
└── examples/
```

## 동작 원리

1. `ar copy` 시점에 메인의 (ignore 제외) 모든 파일을 SHA-256 해시 인덱스 (`.agent/baseline.json`) 로 저장합니다.
2. `ar sync` 는:
   - 사본 vs baseline 차이 → 적용할 변경
   - 메인 vs baseline 차이 → 사용자가 사본 외부에서 만진 변경 (충돌 후보)
   - 두 집합의 교집합이 비어있지 않으면 중단
3. 적용 직전 영향 받는 메인 파일을 `.agent/history/<id>/files/` 로 백업하고, `manifest.json` 에 변경 내역을 기록합니다.
4. `ar undo` 는 manifest 를 읽어 추가→삭제 / 수정·삭제→백업 복원으로 역적용하고 baseline 도 이전 상태로 복원합니다.

## 테스트

```bash
go test ./pkg/...
```
