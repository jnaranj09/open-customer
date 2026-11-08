#!/usr/bin/env bash
# make e2e-docker: run the Playwright e2e tests on the host against WebKit in the pinned Playwright
# image (PLAYWRIGHT_IMAGE, from make). The container oc-e2e-browser runs `playwright run-server`
# (same version as @playwright/test in web/package.json) on the host network, bound to 127.0.0.1,
# so the browser reaches the app on localhost:18080. The container is removed on exit.
set -euo pipefail

cd "$(dirname "$0")/.."

: "${PLAYWRIGHT_IMAGE:?set by make}"
name=oc-e2e-browser
port=${E2E_BROWSER_PORT:-13100}

version=$(sed -n 's/.*"@playwright\/test": "\(.*\)".*/\1/p' web/package.json)
[ -n "$version" ] || {
  echo "e2e-docker: web/package.json has no @playwright/test" >&2
  exit 1
}
if ss -ltn | grep -qE ":($port|18080)\b"; then
  echo "e2e-docker: port $port or 18080 is already in use" >&2
  exit 1
fi

trap 'docker rm -f "$name" >/dev/null 2>&1 || true' EXIT
docker rm -f "$name" >/dev/null 2>&1 || true
docker run -d --name "$name" --network host --init --ipc=host --cap-drop=ALL \
  --security-opt no-new-privileges "$PLAYWRIGHT_IMAGE" \
  npx -y "playwright@$version" run-server --host 127.0.0.1 --port "$port" >/dev/null

# The server prints "Listening on ws://..." when it accepts connections.
for _ in $(seq 1 120); do
  docker logs "$name" 2>&1 | grep -q 'Listening on' && break
  [ "$(docker inspect -f '{{.State.Running}}' "$name")" = true ] || {
    docker logs "$name" >&2
    echo "e2e-docker: the browser server stopped" >&2
    exit 1
  }
  sleep 1
done
docker logs "$name" 2>&1 | grep -q 'Listening on' || {
  docker logs "$name" >&2
  echo "e2e-docker: the browser server did not start" >&2
  exit 1
}
ws=$(docker logs "$name" 2>&1 | sed -n 's/.*Listening on \(ws:[^ ]*\).*/\1/p' | head -1)

cd web
PW_TEST_CONNECT_WS_ENDPOINT="$ws" pnpm e2e
