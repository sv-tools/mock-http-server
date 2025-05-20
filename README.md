# mock-http-server

[![Code Analysis](https://github.com/sv-tools/mock-http-server/actions/workflows/checks.yaml/badge.svg)](https://github.com/sv-tools/mock-http-server/actions/workflows/checks.yaml)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/v/tag/sv-tools/mock-http-server?style=flat)](https://github.com/sv-tools/mock-http-server/releases)

A simple HTTP Server to be used for the unit or end-to-end or integrations tests.
* yaml based configuration, see an [example config file](example_config.yaml).
* docker image is published in this repo and on the docker hub

## Usage

Run docker image with the example config for the latest release:
```shell
docker run -p 8080:8080 -v $(pwd)/example_config.yaml:/config.yaml -e CONFIG=config.yaml ghcr.io/sv-tools/mock-http-server:latest
```

Or build and run the docker image locally:
```shell
GOOS=linux GOARCH=amd64 go build
docker build --tag mock-http-server:latest .
docker run -p 8080:8080 -v $(pwd)/example_config.yaml:/config.yaml -e CONFIG=config.yaml mock-http-server:latest
```

in second shell
```shell
curl http://localhost:8080/users -H"X-Request-Id: 123"
> {"id":1,"name":"John","id":2,"name":"Jane"}

curl http://localhost:8080/users/1 -H"X-Request-Id: 123"
> {"id":1,"name":"John"}
```

The first shell should produce the following log lines:
```json
{"time":"2025-05-20T21:30:14.551460052Z","level":"INFO","msg":"Listen on http://localhost:8080"}
{"time":"2025-05-20T21:31:38.962693466Z","level":"INFO","msg":"request completed","http_scheme":"http","http_proto":"HTTP/1.1","http_method":"GET","remote_addr":"192.168.65.1:18659","user_agent":"curl/8.7.1","uri":"http://localhost:8080/users","resp_status":200,"resp_byte_length":85,"request_id":"123"}
{"time":"2025-05-20T21:32:43.655666343Z","level":"INFO","msg":"request completed","http_scheme":"http","http_proto":"HTTP/1.1","http_method":"GET","remote_addr":"192.168.65.1:47480","user_agent":"curl/8.7.1","uri":"http://localhost:8080/users/1","resp_status":200,"resp_byte_length":34,"request_id":"123"}
```

## License

MIT licensed. See the bundled [LICENSE](LICENSE) file for more details.
