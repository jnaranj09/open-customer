# The one place each tool and Postgres image is pinned (design doc "Pins"). This file is never
# built: the Makefile reads each `FROM <image> AS <name>` line (its tool_image function), and
# Dependabot's docker ecosystem reads it (the /security entry in .github/dependabot.yml), so each pin
# gets its version and digest bump PRs. Every image is <tag>@sha256:<multi-arch index digest>
# (guard TestSupplyChainPins). make config-scan does not scan it, because it is never built.
# A Dependabot PR that bumps postgres or gitleaks here and not its other copies is red on make
# check-pins until those copies change in the same PR.

# The Postgres pin of the stack (db, the backup image's base, make dev, make test-db, the CI e2e
# service). Every copy, digest included, must equal it (make check-pins).
FROM postgres:18.6-alpine@sha256:123b17998ccaf47e53b502e8c6a1e79afd8f579c6f3eec83c563da026f957395 AS postgres

# make generate.
FROM sqlc/sqlc:1.31.1@sha256:70f53171d27b2424e9358869975455a6e955a5aa8e58a998a270a6e34e525537 AS sqlc

# make image-scan, make config-scan.
FROM aquasec/trivy:0.76.1@sha256:790fd66d1697395d8693d1acaa1ea702406a20f109b6fc7bac8db623eca6c06c AS trivy

# make secrets. Its version is also the CI gitleaks action's GITLEAKS_VERSION (make check-pins).
FROM ghcr.io/gitleaks/gitleaks:v8.24.3@sha256:e1b35e12a8c6fa8901f060459cfb6b2fc4c484d3afbe3b029733a3bbfab07055 AS gitleaks

# make config-scan.
FROM ghcr.io/zizmorcore/zizmor:1.31.1@sha256:42d409dbb851d4c5d35a058cf042b828bea1e3f2884378c526eb0974780e18d9 AS zizmor

# make dast.
FROM zaproxy/zap-stable:2.17.0@sha256:9624669db72f54d86c4b46a7064518bd9bf79d8c61a4c31befa9923489f328af AS zap

# make dast (its templates release is NUCLEI_TEMPLATES in the Makefile).
FROM projectdiscovery/nuclei:v3.11.1@sha256:582d5546902e67052097cb2d07296c642d50a1afc5e44623cb038845df9a32eb AS nuclei

# make dast: drives the app (setup, login, the session check).
FROM curlimages/curl:8.22.0@sha256:58adaa4e8dca9c988bae2aba4ab3434a0bb2da16bbe3f92dec39ec7785166777 AS curl

# make sast: runs only the rules in security/semgrep.
FROM semgrep/semgrep:1.182.0@sha256:4a34048c04fef049f71665047f074cffb00cc3d89482f965a5cd151d589237af AS semgrep
