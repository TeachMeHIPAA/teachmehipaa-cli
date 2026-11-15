# Used by GoReleaser (dockers_v2), which puts each platform's prebuilt tmh
# under $TARGETPLATFORM/ in the build context; it is not a source build.
# distroless/static has CA certificates and no shell, and runs as nonroot.
FROM gcr.io/distroless/static-debian13:nonroot@sha256:aafe993665bf6200db38a02c11a9351e782a84b6a45e7f33b6695638c88e3931
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/tmh /usr/local/bin/tmh
ENTRYPOINT ["/usr/local/bin/tmh"]
