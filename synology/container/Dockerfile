# Reuse the released multi-platform system/tool layer: Node 22.23.4, npm,
# @lydell/node-pty 1.1.0 and the existing Ubuntu tools. All application payload
# files are replaced below; the immutable base is also the previous SPK's image.
FROM ghcr.io/synopackageland/syno-pkg-land-ovis-runtime:0.5.1-0001@sha256:68603f08ecea980c5a2c693be5c782e5bc9741a1da49c32f439908541117c711 AS tools
FROM tools
USER 0:0
ARG TARGETARCH
# sudo inside the container (SDK run_as.sudo): the console user `ovis` may run
# single commands as container root, for example `sudo apt-get install`. The
# SDK's init appends that user's /etc/passwd and /etc/group entry for the
# package UID at start, because setuid sudo reads only the real files. tini is
# PID 1 and reaps processes orphaned by terminals and agents.
RUN apt-get update \
    && apt-get install -y --no-install-recommends sudo tini \
    && rm -rf /var/lib/apt/lists/* /opt/ovis \
    && printf 'ovis ALL=(ALL:ALL) NOPASSWD: ALL\n' > /etc/sudoers.d/ovis \
    && chmod 0440 /etc/sudoers.d/ovis \
    && chmod 0666 /etc/passwd /etc/group
COPY payload/${TARGETARCH}/ /opt/ovis/
# Fresh source preparation prunes the Node addon. Restore only the unchanged,
# target-native PTY packages from the verified tools layer.
COPY --from=tools /opt/ovis/app/node_modules/@lydell/ /opt/ovis/app/node_modules/@lydell/
COPY LICENSES/ /usr/share/doc/ovis-runtime/
ENV HOME=/data/home
USER 1000:1000
ENTRYPOINT ["/usr/bin/tini", "--", "/opt/ovis/ovis-launch"]
