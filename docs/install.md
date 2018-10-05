# Install DéMars

## From Source

You'll need `go` [installed](https://golang.org/doc/install) and the required
[environment variables set](https://github.com/golang/go/wiki/SettingGOPATH)

### Get Source Code

```
mkdir -p $GOPATH/src/github.com/Demars-DMC
cd $GOPATH/src/github.com/Demars-Dmc
git clone https://github.com/Demars-DMC/Demars-DMC.git
cd Demars-DMC
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

The latest `Demars-DMC version` is now installed.

## Reinstall

If you already have DéMars installed, and you make updates, simply

```
cd $GOPATH/src/github.com/Demars-DMC/Demars-DMC
make install
```

To upgrade, run

```
cd $GOPATH/src/github.com/Demars-DMC/Demars-DMC
git pull origin master
make get_vendor_deps
make install
```

## Run

To start a one-node blockchain with a simple in-process application:

```
Demars-DMC init
Demars-DMC node --proxy_app=dmccoin
```
