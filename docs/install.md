# Install DéMars

## From Source

You'll need `go` [installed](https://golang.org/doc/install) and the required
[environment variables set](https://github.com/golang/go/wiki/SettingGOPATH)

### Get Source Code

```
mkdir -p $GOPATH/src/github.com/demars-dmc
cd $GOPATH/src/github.com/demars-dmc
git clone https://github.com/demars-dmc/demars-dmc.git
cd demars-dmc
```

### Get Tools & Dependencies

```
make get_tools
make get_vendor_deps
```

### Compile

```
make install
```

to put the binary in `$GOPATH/bin` or use:

```
make build
```

to put the binary in `./build`.

The latest `demars version` is now installed.

## Reinstall

If you already have DéMars installed, and you make updates, simply

```
cd $GOPATH/src/github.com/demars-dmc/demars-dmc
make install
```

To upgrade, run

```
cd $GOPATH/src/github.com/demars-dmc/demars-dmc
git pull origin master
make get_vendor_deps
make install
```

## Run

To start a one-node blockchain with a simple in-process application:

```
demars init
demars node --proxy_app=dmccoin
```
