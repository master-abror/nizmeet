#!/usr/bin/env bash
# Menyalakan dev infra lokal (PostgreSQL 18 + Redis) dan memverifikasinya.
# Dipanggil oleh `make dev-up`. Aman dijalankan berulang.
set -euo pipefail

DEPLOY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$DEPLOY_DIR/.env"
ENV_EXAMPLE="$DEPLOY_DIR/.env.example"
PLACEHOLDER="GANTI_DENGAN_PASSWORD_LOKAL"

die() { echo "dev-up: $*" >&2; exit 1; }

command -v docker >/dev/null 2>&1 || die "docker tidak ditemukan di PATH."
docker info >/dev/null 2>&1 || die "daemon Docker tidak berjalan (nyalakan Docker Desktop)."
docker compose version >/dev/null 2>&1 || die "plugin 'docker compose' tidak tersedia."

random_password() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 16
  else
    od -An -N16 -tx1 /dev/urandom | tr -d ' \n'
  fi
}

if [ ! -f "$ENV_FILE" ]; then
  [ -f "$ENV_EXAMPLE" ] || die "$ENV_EXAMPLE tidak ditemukan."
  pw="$(random_password)"
  sed "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=${pw}|" "$ENV_EXAMPLE" > "$ENV_FILE"
  echo "dev-up: membuat deploy/.env dengan password acak (tidak ter-commit)."
fi

if grep -q "^POSTGRES_PASSWORD=${PLACEHOLDER}\$" "$ENV_FILE"; then
  die "deploy/.env masih berisi password contoh; ganti POSTGRES_PASSWORD."
fi

# shellcheck disable=SC1090
set -a; . "$ENV_FILE"; set +a
PG_PORT="${POSTGRES_PORT:-5432}"
REDIS_PORT="${REDIS_PORT:-6379}"

COMPOSE=(docker compose --project-directory "$DEPLOY_DIR" --env-file "$ENV_FILE" -f "$DEPLOY_DIR/docker-compose.yml")

if ! "${COMPOSE[@]}" up -d --wait; then
  echo "dev-up: gagal menyalakan layanan." >&2
  echo "  Bila port bentrok (mis. kontainer manual pg18 di 5432):" >&2
  echo "  jalankan 'docker rm -f pg18' atau ubah POSTGRES_PORT/REDIS_PORT di deploy/.env." >&2
  exit 1
fi

echo "dev-up: verifikasi..."
"${COMPOSE[@]}" exec -T postgres pg_isready -h 127.0.0.1 -p 5432 -U "$POSTGRES_USER" -d "$POSTGRES_DB"
"${COMPOSE[@]}" exec -T redis redis-cli ping

echo "dev-up: siap."
echo "  PostgreSQL : postgres://${POSTGRES_USER}:***@127.0.0.1:${PG_PORT}/${POSTGRES_DB}"
echo "  Redis      : redis://127.0.0.1:${REDIS_PORT}"
