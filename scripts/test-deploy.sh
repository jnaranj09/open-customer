#!/usr/bin/env bash
# make test-deploy (design doc item 2): builds the backup image from deploy/backup and runs
# deploy/tests/backup_test.sh inside it, against a throwaway Postgres (PG_IMAGE) on a private
# network, then deploy/tests/scripts_test.sh (the operator scripts, with a fake docker) on the
# host. No host port is published. The containers and the network are named oc-deploy-* and
# removed on exit, pass or fail, with their anonymous volumes: the Postgres image (and so the
# backup image) declares VOLUME /var/lib/postgresql, and `docker rm -f` alone would leave one
# volume behind per container and run.
set -euo pipefail

cd "$(dirname "$0")/.."

: "${PG_IMAGE:?set by make}"

image=open-customer-backup:test
net=oc-deploy-net
pg=oc-deploy-pg
runner=oc-deploy-backup-test
# The throwaway database's password; this container lives only for this run.
pg_password=test-only

cleanup() {
  docker rm -f -v "$runner" "$pg" >/dev/null 2>&1 || true
  docker network rm "$net" >/dev/null 2>&1 || true
}
trap cleanup EXIT
# A run killed before its trap ran leaves these behind.
cleanup

docker build -q -t "$image" deploy/backup >/dev/null
docker network create "$net" >/dev/null
docker run -d --name "$pg" --network "$net" \
  -e POSTGRES_USER=open_customer -e POSTGRES_DB=open_customer -e POSTGRES_PASSWORD="$pg_password" \
  "$PG_IMAGE" >/dev/null

# Over TCP: on a first start the image's init server answers on the socket only, then restarts.
tries=0
until docker exec "$pg" pg_isready -q -h 127.0.0.1 -U open_customer -d open_customer; do
  tries=$((tries + 1))
  if [ "$tries" -ge 60 ]; then
    echo "test-deploy: $pg did not become ready in 60 s" >&2
    exit 1
  fi
  sleep 1
done

docker run --rm --name "$runner" --network "$net" \
  -v "$PWD/deploy:/deploy:ro" \
  -e PGHOST="$pg" -e PGPORT=5432 -e PGUSER=open_customer -e PGPASSWORD="$pg_password" \
  -e PGDATABASE=open_customer \
  --entrypoint bash "$image" /deploy/tests/backup_test.sh

bash deploy/tests/scripts_test.sh
