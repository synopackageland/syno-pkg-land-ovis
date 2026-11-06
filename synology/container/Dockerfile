# Fetch target-native Node/npm and PTY prebuilds without executing target code.
FROM --platform=$BUILDPLATFORM public.ecr.aws/docker/library/node:22.23.4-bookworm-slim@sha256:5ec23b7c622b00547753bc6ffd78085dd868a68aeb41c8e0a1418f033e13d96d AS pty
ARG TARGETARCH
ARG https_proxy
RUN case "$TARGETARCH" in amd64) cpu=x64 ;; arm64) cpu=arm64 ;; *) exit 1 ;; esac \
    && npm install --prefix /pty --cpu="$cpu" --os=linux --omit=dev --ignore-scripts @lydell/node-pty@1.1.0
FROM public.ecr.aws/docker/library/node:22.23.4-bookworm-slim@sha256:5ec23b7c622b00547753bc6ffd78085dd868a68aeb41c8e0a1418f033e13d96d AS node
# Ubuntu supplies glibc for Bun, Node and agent installer binaries.
FROM public.ecr.aws/docker/library/ubuntu:24.04@sha256:fe81560990fae86dc4743202b2cb4d3df98d0bcf949173cecef15ce1127ef852
ARG TARGETARCH
# bash: panes/installers; git/openssh-client: repositories and remote PCs;
# curl/CA: HTTPS installers; unzip/xz: common binary archives;
# procps: agent process inspection. No compiler, daemon, sudo or Docker client.
RUN sed -i 's/^Components:.*/Components: main/' /etc/apt/sources.list.d/ubuntu.sources \
    && apt-get update && apt-get install -y --no-install-recommends \
    bash git openssh-client curl ca-certificates unzip xz-utils procps \
    && rm -rf /var/lib/apt/lists/*
COPY --from=node /usr/local/bin/node /usr/local/bin/node
COPY --from=node /usr/local/lib/node_modules/npm/ /usr/local/lib/node_modules/npm/
COPY payload/${TARGETARCH}/ /opt/ovis/
COPY --from=pty /pty/node_modules/ /opt/ovis/app/node_modules/
RUN ln -s ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
    && ln -s ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx \
    && ln -s /opt/ovis/bun /usr/local/bin/bun \
    && ln -s /opt/ovis/herdr /usr/local/bin/herdr \
    && mkdir /data && chmod 1777 /data
COPY LICENSES/ /usr/share/doc/ovis-runtime/
ENV HOME=/data/home
USER 1000:1000
ENTRYPOINT ["/opt/ovis/ovis-launch"]
