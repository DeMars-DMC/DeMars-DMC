# Docker
## How to use this image

### Start one instance of the DéMars with the `dmccoin` app

A quick example of a built-in app and DéMars in one container.

```
docker run -it --rm -v "/tmp:/demars" demars-dmc/demars init
docker run -it --rm -v "/tmp:/demars" demars-dmc/demars node --proxy_app=kvstore
```

## Local cluster

To run a 4-node network, see the `Makefile` in the root of [the repo](https://github.com/demars-dmc/demars-dmc/Makefile) and run:

```
make build-linux
make build-docker-localnode
make localnet-start
```

Note that this will build and use a different image than the ones provided here.
