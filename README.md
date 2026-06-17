# phantom-exporter

테스트/개발용 **가상 Prometheus Exporter 시뮬레이터**.
URL별로 가짜 metric을 정의하고, 값을 실시간으로 제어하며 Prometheus가 스크랩할 수 있게 합니다.

## 주요 기능

- **gin** 기반 fullstack 단일 바이너리 (REST API + HTML GUI + `/metrics`)
- **slog** 기반 파일 로그 (stdout 미러링)
- 메트릭/설정값은 **PostgreSQL**에 저장, 접속 정보는 **환경변수**
- 웹 GUI에서 URL(그룹)별 메트릭 CRUD, 실시간 값 제어, 서비스/사용 현황 모니터링
- **Docker** 기반 실행, VSCode/Cursor **F5** 디버깅 지원

## 디렉토리 구조

```
cmd/exporter/      진입점 (main.go)
internal/config/   환경변수 로딩
internal/logging/  slog 파일 로거
internal/metrics/  도메인 모델 + 값 생성기 + Prometheus 텍스트 렌더링
internal/store/    PostgreSQL 저장소 (스키마/CRUD)
internal/server/   gin 라우터/핸들러/미들웨어 + 사용 통계
web/               HTML/CSS/JS GUI (embed로 바이너리에 포함)
```

## 환경변수

| 변수 | 기본값 | 설명 |
|------|--------|------|
| `PORT` | `8080` | HTTP 포트 |
| `LOG_FILE` | `./logs/phantom-exporter.log` | 로그 파일 경로 |
| `LOG_LEVEL` | `info` | `debug`/`info`/`warn`/`error` |
| `DATABASE_URL` | (없음) | 설정 시 아래 `DB_*`보다 우선 |
| `DB_HOST` | `localhost` | PostgreSQL 호스트 |
| `DB_PORT` | `5432` | 포트 |
| `DB_USER` | `phantom` | 사용자 |
| `DB_PASSWORD` | `phantom` | 비밀번호 |
| `DB_NAME` | `phantom` | DB 이름 |
| `DB_SSLMODE` | `disable` | SSL 모드 |

## 실행 (Docker)

```bash
docker compose up --build
```

- GUI: http://localhost:8080/
- 스크랩 예: http://localhost:8080/metrics/<group-path>

## 디버깅 (F5)

`.vscode/launch.json`이 포함되어 있습니다. 로컬 PostgreSQL을 띄운 뒤 VSCode/Cursor에서 **F5**로 디버깅합니다.

```bash
# 로컬 DB만 컨테이너로 기동
docker run -d --name phantom-pg -e POSTGRES_USER=phantom \
  -e POSTGRES_PASSWORD=phantom -e POSTGRES_DB=phantom -p 5432:5432 postgres:16
```

## API 요약

| 메서드 | 경로 | 설명 |
|--------|------|------|
| GET | `/api/status` | 서비스/DB 현황 |
| GET | `/api/stats` | 사용 통계(스크랩 수 등) |
| GET/POST | `/api/groups` | URL 그룹 목록/생성 |
| GET/PUT/DELETE | `/api/groups/:id` | 그룹 조회/수정/삭제 |
| POST | `/api/groups/:id/metrics` | 메트릭 추가 |
| GET/PUT/DELETE | `/api/metrics-def/:mid` | 메트릭 조회/수정/삭제 |
| POST | `/api/metrics-def/:mid/value` | 실시간 값 오버라이드(`{"value": n}` / `null`=해제) |
| GET | `/metrics/:path` | Prometheus 스크랩 |

## 값 생성 방식 (valueMode)

- `random`: 0~1 랜덤
- `range`: `[minValue, maxValue]` 범위 랜덤
- `fixed`: `fixedValue` 고정
- `counter` 타입은 매 스크랩마다 단조 증가합니다.
