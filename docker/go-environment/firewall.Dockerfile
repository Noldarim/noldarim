FROM debian:trixie-slim

ARG TZ
ENV TZ="$TZ"

RUN apt-get update && apt-get install -y \
    less \
    procps \
    sudo \
    unzip \
    gnupg \
    iptables \
    ipset \
    iproute2 \
    dnsutils \
    aggregate \
    curl \
    jq \
    && rm -rf /var/lib/apt/lists/*

COPY init-firewall.sh /usr/local/bin/
RUN chmod 0555 /usr/local/bin/init-firewall.sh