# mock-http-server

[![Code Analysis](https://github.com/sv-tools/mock-http-server/actions/workflows/checks.yaml/badge.svg)](https://github.com/sv-tools/mock-http-server/actions/workflows/checks.yaml)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/v/tag/sv-tools/mock-http-server?style=flat)](https://github.com/sv-tools/mock-http-server/releases)

A simple HTTP Server to be used for the unit or end-to-end or integrations tests.
* yaml based configuration, see an [example config file](example_config.yaml).
* docker image is published in this repo and on the docker hub

## Usage

```shell
docker run -p 8080:8080 -v $(pwd)/example_config.yaml:/config.yaml -e CONFIG=config.yaml ghcr.io/sv-tools/mock-http-server:latest
```

in second shell
```shell
curl http://localhost:8080/users -H"X-Request-Id: 123"
> {"id":1,"name":"John","id":2,"name":"Jane"}

curl http://localhost:8080/users/1 -H"X-Request-Id: 123"
> {"id":1,"name":"John"}
```


## License

MIT licensed. See the bundled [LICENSE](LICENSE) file for more details.
