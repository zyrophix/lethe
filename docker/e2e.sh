#!/usr/bin/env bash
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="lethe-e2e:latest"
CONTAINER="lethe-e2e-$(date +%s)"
MARKER="LETHE_E2E_MARKER_$RANDOM"
LOG="$ROOT/docker/e2e.log"

MIN_AVAILABLE_MB=2048
SAMPLER_PID=""

log() {
    echo "[$(date +%H:%M:%S)] $*" | tee -a "$LOG"
}

mem_line() {
    free -m | awk '/Mem:/{printf "host mem: total=%dMB used=%dMB avail=%dMB swap=%dMB", $2, $3, $7, $8}'
}

dexec() {
    log "EXEC: docker exec $CONTAINER $*"
    docker exec "$CONTAINER" "$@" 2>&1 | tee -a "$LOG"
    local rc=${PIPESTATUS[0]}
    log "EXEC rc=$rc"
    return $rc
}

sampler() {
    while true; do
        local host_mem avail
        avail=$(free -m | awk '/Mem:/{print $7}')
        host_mem=$(free -m | awk '/Mem:/{printf "%dMB avail / %dMB used", $7, $3}')
        log "SAMPLER: host $host_mem | container $(docker stats --no-stream --format '{{.MemUsage}} ({{.MemPerc}})' "$CONTAINER" 2>/dev/null || echo 'n/a')"
        if [ "$avail" -lt "$MIN_AVAILABLE_MB" ]; then
            log "WARNING: host available memory dropped below ${MIN_AVAILABLE_MB}MB!"
        fi
        sleep 5
    done
}

cleanup() {
    if [ -n "$SAMPLER_PID" ]; then
        kill "$SAMPLER_PID" 2>/dev/null || true
    fi
    log "cleanup: removing container $CONTAINER"
    docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
    log "cleanup done"
}
trap cleanup EXIT

cd "$ROOT"
: > "$LOG"
log "=== E2E START (marker=$MARKER) ==="
log "host: $(uname -a)"
log "mem: $(mem_line)"

if ! docker info >/dev/null 2>&1; then
    log "FATAL: docker not available"
    exit 1
fi

AVAIL_MB=$(free -m | awk '/Mem:/{print $7}')
if [ "$AVAIL_MB" -lt "$MIN_AVAILABLE_MB" ]; then
    log "FATAL: only ${AVAIL_MB}MB available, need >= ${MIN_AVAILABLE_MB}MB. Aborting to protect the system."
    exit 1
fi
log "available memory OK (${AVAIL_MB}MB)"

log "==> building linux/amd64 binary"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o dist/lethe-linux-amd64 ./cmd/lethe 2>&1 | tee -a "$LOG"
log "build done"

log "==> building e2e image"
docker build -t "$IMAGE" -f docker/Dockerfile . 2>&1 | tee -a "$LOG"
log "image built"

log "==> starting container (caps, mem limit 512m, 2 cpus)"
docker run -d \
    --name "$CONTAINER" \
    --cap-add AUDIT_CONTROL \
    --cap-add AUDIT_WRITE \
    --cap-add SYS_ADMIN \
    --memory 512m \
    --cpus 2 \
    --pids-limit 256 \
    "$IMAGE" >/dev/null 2>&1
log "container started: $CONTAINER"
sampler &
SAMPLER_PID=$!
sleep 3

log "==> starting auditd (no systemd)"
dexec bash -c "service auditd start 2>&1 || true"
sleep 2

log "==> creating marked artifacts"
dexec bash -c "
  echo 'Jul 01 12:00:00 host sshd[1234]: Failed password for root from 1.2.3.4 port 22 ssh2 $MARKER' >> /var/log/auth.log
  mkdir -p /root/.ssh && echo '127.0.0.1 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIx $MARKER' > /root/.ssh/known_hosts
  chmod 700 /root/.ssh
  mkdir -p /var/log/audit && echo '$MARKER audit entry' >> /var/log/audit/audit.log
  echo '$MARKER syslog entry' >> /var/log/syslog
  echo '#!/bin/sh' > /tmp/lethe_e2e_${MARKER}.sh && echo 'echo $MARKER' >> /tmp/lethe_e2e_${MARKER}.sh && chmod +x /tmp/lethe_e2e_${MARKER}.sh
  ls -la /tmp/lethe_e2e_${MARKER}.sh
"
log "mem: $(mem_line)"

log "==> verifying markers exist before clean"
dexec bash -c "grep -q '$MARKER' /var/log/auth.log && echo 'AUTH_LOG_MARKER OK' || echo 'AUTH_LOG_MARKER MISSING'"
dexec bash -c "grep -q '$MARKER' /root/.ssh/known_hosts && echo 'KNOWN_HOSTS MARKER OK' || echo 'KNOWN_HOSTS MARKER MISSING'"
dexec bash -c "grep -q '$MARKER' /var/log/audit/audit.log && echo 'AUDIT_LOG_MARKER OK' || echo 'AUDIT_LOG_MARKER MISSING'"
dexec bash -c "grep -q '$MARKER' /var/log/syslog && echo 'SYSLOG_MARKER OK' || echo 'SYSLOG_MARKER MISSING'"
dexec bash -c "test -f /tmp/lethe_e2e_${MARKER}.sh && echo 'TEMP_MARKER OK' || echo 'TEMP_MARKER MISSING'"

log "==> running lethe clean"
dexec bash -c "lethe clean --force --max-risk risky --modules ssh,audit,logs,temp 2>&1 || true"
log "mem: $(mem_line)"

log "==> verifying markers gone"
FAILED=0

if dexec bash -c "grep -q '$MARKER' /var/log/auth.log 2>/dev/null"; then
    log "FAIL: auth.log still contains marker"
    FAILED=1
else
    log "PASS: auth.log marker removed"
fi

if dexec bash -c "grep -q '$MARKER' /root/.ssh/known_hosts 2>/dev/null"; then
    log "FAIL: known_hosts still contains marker"
    FAILED=1
else
    log "PASS: known_hosts marker removed"
fi

if dexec bash -c "grep -q '$MARKER' /var/log/audit/audit.log 2>/dev/null"; then
    log "FAIL: audit.log still contains marker"
    FAILED=1
else
    log "PASS: audit.log marker removed"
fi

if dexec bash -c "grep -q '$MARKER' /var/log/syslog 2>/dev/null"; then
    log "FAIL: syslog still contains marker"
    FAILED=1
else
    log "PASS: syslog marker removed"
fi

if dexec bash -c "test -f /tmp/lethe_e2e_${MARKER}.sh 2>/dev/null"; then
    log "FAIL: temp marker file still exists"
    FAILED=1
else
    log "PASS: temp marker file removed"
fi

log "mem: $(mem_line)"
if [ "$FAILED" -eq 1 ]; then
    log "E2E FAILED"
    exit 1
fi

log "E2E PASSED"
log "docker system df:"
docker system df 2>&1 | tee -a "$LOG"