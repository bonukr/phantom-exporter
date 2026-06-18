# phantom-exporter

테스트/개발용 **가상 Prometheus Exporter 시뮬레이터**.
URL별로 가짜 metric을 정의하고, 값을 실시간으로 제어하며 Prometheus가 스크랩할 수 있게 합니다.

## 주요 기능

- **gin** 기반 fullstack 단일 바이너리 (REST API + HTML GUI + `/metrics`)
- **slog** 기반 파일 로그 (stdout 미러링)
- 메트릭/설정값은 **`settings/` 폴더에 그룹별 YAML 파일**로 저장 (DB 불필요). GUI에서 변경하면 파일로 자동 저장
- 웹 GUI에서 URL(그룹)별 메트릭 CRUD, 실시간 값 제어, 서비스/사용 현황 모니터링
- 하단 **Scrape Test** 패널에서 선택한 metric URL을 실제 호출해 응답 확인
- **Docker** 기반 실행, VSCode/Cursor **F5** 디버깅 지원

## 디렉토리 구조

```
cmd/exporter/      진입점 (main.go)
internal/config/   환경변수 로딩
internal/logging/  slog 파일 로거
internal/metrics/  도메인 모델 + 값 생성기 + Prometheus 텍스트 렌더링
internal/store/    settings 폴더 저장소 (그룹별 파일 로드/저장/CRUD)
internal/server/   gin 라우터/핸들러/미들웨어 + 사용 통계
web/               HTML/CSS/JS GUI (embed로 바이너리에 포함)
settings.example/  설정 파일 예시 (그룹별 YAML)
```

## 환경변수

| 변수 | 기본값 | 설명 |
|------|--------|------|
| `H2H_PORT` | `8080` | HTTP 포트 |
| `H2H_SETTINGS_DIR` | `./settings` | 그룹별 YAML 파일이 담긴 폴더 (비어 있으면 기본값으로 자동 생성) |
| `H2H_LOG_FILE` | `./logs/phantom-exporter.log` | 로그 파일 경로 |
| `H2H_LOG_LEVEL` | `info` | `debug`/`info`/`warn`/`error` |

## 설정 (settings 폴더)

- `H2H_SETTINGS_DIR` 폴더 안에 **그룹마다 하나의 YAML 파일**을 둡니다. 파일명은 그룹 path를 사용합니다: `settings/<path>.yml` → `/metrics/<path>`로 노출.
- 폴더가 비어 있으면 데모 그룹(`settings/demo.yml`)이 자동 생성됩니다.
- GUI/API로 변경한 내용은 해당 그룹 파일에만 원자적으로(temp + rename) 다시 기록됩니다. 그룹 path 변경 시 파일도 함께 이름이 바뀝니다.
- 형식은 `settings.example/demo.yml`을 참고하세요.

## 실행 (Docker)

```bash
docker compose up --build
```

- GUI: http://localhost:8080/
- 스크랩 예: http://localhost:8080/metrics/demo
- 설정/로그는 호스트의 `./settings`, `./logs`에 볼륨으로 보존됩니다.

## 디버깅 (F5)

`.vscode/launch.json`이 포함되어 있습니다. DB 없이 VSCode/Cursor에서 **F5**로 바로 디버깅할 수 있습니다.

## API 요약

| 메서드 | 경로 | 설명 |
|--------|------|------|
| GET | `/api/status` | 서비스/설정 현황 |
| GET | `/api/stats` | 사용 통계(스크랩 수 등) |
| GET/POST | `/api/groups` | URL 그룹 목록/생성 |
| GET/PUT/DELETE | `/api/groups/:id` | 그룹 조회/수정/삭제 |
| POST | `/api/groups/:id/metrics` | 메트릭 추가 |
| GET/PUT/DELETE | `/api/metrics-def/:mid` | 메트릭 조회/수정/삭제 |
| POST | `/api/metrics-def/:mid/value` | 실시간 값 오버라이드(`{"value": n}` / `null`=해제) |
| GET | `/metrics/:path` | Prometheus 스크랩 |

## 값 생성 방식 (valueMode)

| 모드 | 동작 | 사용 필드 |
|------|------|-----------|
| `random` | 0~1 랜덤 | - |
| `range` | `[minValue, maxValue]` 범위 균등 랜덤 | min, max |
| `fixed` | `fixedValue` 고정 | fixed |
| `increment` | 매 스크랩 `step`씩 증가(기본 1), 무한 | min(시작값), step |
| `ramp` | 매 스크랩 `step`씩 증가하다 max 초과 시 min으로 복귀(톱니) | min, max, step |
| `step` | `[min,max]`를 `step`개 레벨로 나눠 매 스크랩 한 단계씩 순환(계단) | min, max, step(레벨 수) |
| `rate` | **경과 시간** 기준 초당 `step`만큼 선형 증가/감소(`step<0`이면 감소) | min(시작값), step(초당 변화량) |

- `increment`/`ramp`/`step`은 **스크랩 횟수** 기준, `rate`는 **실제 경과 시간(초)** 기준입니다.
- `counter` 타입은 위 모드와 별개로 기본적으로 매 스크랩 단조 증가합니다.
