# hrobot-go: A Go library for the Hetzner Robot Webservice

Package hrobot-go is a library for the Hetzner Robot Webservice.

The library’s documentation is available at [GoDoc](https://godoc.org/github.com/syself/hrobot-go),
the public API documentation is available at [robot.your-server.de](https://robot.your-server.de/doc/webservice/en.html).

## Infos about fork

This fork is based on the repo of nl2go. The original repo implemented the important parts of Hetzner Robot API, but has not been updated since 2019. This work has the goal to keep up with API changes on Hetzner side and to implement additional functions that Hetzner Robot offers.

Contributions and feature requests are very welcome!

## Example

```go
package main

import (
    "fmt"
    "log"

    client "github.com/syself/hrobot-go"
)

func main() {
    robotClient := client.NewBasicAuthClient("user", "pass")

    servers, err := robotClient.ServerGetList()
    if err != nil {
        log.Fatalf("error while retrieving server list: %s\n", err)
    }

    fmt.Println(servers)
}
```

If you want to add instrumentation (for example to debug why you hit rate-limits of the Hetzner API)
you can use `NewBasicAuthClientWithCustomHttpClient()` to use your own httpClient.

## Releasing

Update version number in `client.go`

```sh
make test

export RELEASE_TAG=vX.Y.Z

git tag -a ${RELEASE_TAG} -m ${RELEASE_TAG}

git push origin $RELEASE_TAG
```
