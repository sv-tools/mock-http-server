FROM scratch
ENV CONFIG=config.yaml
ENV PORT=8080
ENTRYPOINT ["/mock-http-server"]
COPY mock-http-server /
