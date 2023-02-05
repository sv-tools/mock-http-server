FROM scratch
ENV CONFIG=config.yaml
ENTRYPOINT ["/mock-http-server"]
COPY mock-http-server /
