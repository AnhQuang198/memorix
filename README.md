# Memorix

Web app học từ vựng tiếng Anh bằng FSRS. Go modular monolith + React.

## Chạy local
```bash
docker compose up -d --build
curl localhost:8080/api/v1/health   # {"status":"ok"}
```

## Bố cục
```
backend/   # Go modular monolith (cmd, internal, migrations, db, go.mod)
web/       # React + Vite frontend
_bmad-output/, docs/, design/  # planning artifacts
```

## Test
```bash
cd backend && go test ./...      # backend (bỏ -short để chạy testcontainers, cần Docker)
cd web && npx vitest run         # frontend
cd backend && golangci-lint run ./...  # lint + enforce ranh giới (depguard)
```

## Cấu trúc backend
Modular Monolith + Hexagonal core. Module = bounded context dưới `backend/internal/`;
ruột module ở `backend/internal/<module>/internal/` (compiler chặn import chéo);
depguard chặn `domain` import framework/hạ tầng (AD-2).
Chi tiết: `_bmad-output/planning-artifacts/architecture/architecture-memorix-2026-07-07/addendum-structure.md`.
