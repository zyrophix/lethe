FROM ubuntu:22.04

RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    systemd systemd-sysv auditd openssh-server rsyslog \
    && rm -rf /var/lib/apt/lists/*

RUN systemctl enable auditd 2>/dev/null || true

COPY dist/lethe-linux-amd64 /usr/local/bin/lethe
RUN chmod +x /usr/local/bin/lethe

CMD ["bash", "-c", "service auditd start; sleep infinity"]