# Reuse the released multi-platform system/tool layer: Node 22.23.4, npm,
# @lydell/node-pty 1.1.0 and the existing Ubuntu tools. All application payload
# files are replaced below; the immutable base is also the previous SPK's image.
FROM ghcr.io/synopackageland/syno-pkg-land-ovis-runtime:0.5.1-0001@sha256:68603f08ecea980c5a2c693be5c782e5bc9741a1da49c32f439908541117c711 AS tools
FROM tools
USER 0:0
ARG TARGETARCH
# build-image.sh verifies these official Ubuntu archives against the committed
# SHA-256 pins. This library needs no package scripts or privileged runtime step.
COPY support/libnss-wrapper_${TARGETARCH}.deb /tmp/libnss-wrapper.deb
RUN dpkg-deb --extract /tmp/libnss-wrapper.deb / \
    && rm /tmp/libnss-wrapper.deb \
    && rm -rf /opt/ovis \
    && ln -s /usr/lib/$(uname -m)-linux-gnu/libnss_wrapper.so /usr/local/lib/libnss_wrapper.so
COPY payload/${TARGETARCH}/ /opt/ovis/
# Fresh source preparation prunes the Node addon. Restore only the unchanged,
# target-native PTY packages from the verified tools layer.
COPY --from=tools /opt/ovis/app/node_modules/@lydell/ /opt/ovis/app/node_modules/@lydell/
COPY LICENSES/ /usr/share/doc/ovis-runtime/
ENV HOME=/data/home
USER 1000:1000
ENTRYPOINT ["/opt/ovis/ovis-launch"]
