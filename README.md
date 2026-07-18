# Memorix

Web app học từ vựng tiếng Anh bằng FSRS. Go modular monolith + React.

## Chạy local
```bash
docker compose up -d --build
curl localhost:8080/api/v1/health   # {"status":"ok"}
```

## Test
```bash
go test ./...            # backend (bỏ -short để chạy testcontainers, cần Docker)
cd web && npx vitest run # frontend
golangci-lint run ./...  # lint + enforce ranh giới (depguard)
```

## Cấu trúc
Modular Monolith + Hexagonal core. Module = bounded context dưới `internal/`;
ruột module ở `internal/<module>/internal/` (compiler chặn import chéo);
depguard chặn `domain` import framework/hạ tầng (AD-2).
Chi tiết: `_bmad-output/planning-artifacts/architecture/architecture-memorix-2026-07-07/addendum-structure.md`.
