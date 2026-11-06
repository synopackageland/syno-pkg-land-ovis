# All source compilation runs in Docker on the Linux build host.
FROM public.ecr.aws/docker/library/golang:1.26.10-bookworm@sha256:5de7c99eb78ca5e515a4c340235d0955ec5964a5071c492c15503d7c267da473 AS prepare
ARG https_proxy
RUN apt-get update && apt-get install -y --no-install-recommends unzip \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY source/ ./
COPY packaging/ ./synology/
# The source hook exports a committed snapshot, so give this private build copy
# its own index. No host Git config, credentials or worktree paths are copied.
RUN git init -q && git add -A \
    && git -c user.name=Builder -c user.email=builder@invalid commit -qm 'Build source snapshot' \
    && bash synology/build-source.sh \
    && mkdir -p /out/helpers/x86_64 /out/helpers/aarch64 \
    && cp -R synology/daemon/x86_64 /out/amd64 \
    && cp -R synology/daemon/aarch64 /out/arm64 \
    && cp synology/daemon/x86_64/ovis-token /out/helpers/x86_64/ \
    && cp synology/daemon/aarch64/ovis-token /out/helpers/aarch64/
FROM scratch AS payload
COPY --from=prepare /out/ /
