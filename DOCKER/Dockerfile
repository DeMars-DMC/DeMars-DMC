FROM alpine:3.7
MAINTAINER Greg Szabo <greg@Demars-DMC.com>

# Demars-DMC will be looking for the genesis file in /Demars-DMC/config/genesis.json
# (unless you change `genesis_file` in config.toml). You can put your config.toml and
# private validator file into /Demars-DMC/config.
#
# The /Demars-DMC/data dir is used by Demars-DMC to store state.
ENV TMHOME /Demars-DMC

# OS environment setup
# Set user right away for determinism, create directory for persistence and give our user ownership
# jq and curl used for extracting `pub_key` from private validator while
# deploying Demars-DMC with Kubernetes. It is nice to have bash so the users
# could execute bash commands.
RUN apk update && \
    apk upgrade && \
    apk --no-cache add curl jq bash && \
    addgroup tmuser && \
    adduser -S -G tmuser tmuser -h "$TMHOME"

# Run the container with tmuser by default. (UID=100, GID=1000)
USER tmuser

# Expose the data directory as a volume since there's mutable state in there
VOLUME [ $TMHOME ]

WORKDIR $TMHOME

# p2p and rpc port
EXPOSE 26656 26657

ENTRYPOINT ["/usr/bin/Demars-DMC"]
CMD ["node", "--moniker=`hostname`"]
STOPSIGNAL SIGTERM

ARG BINARY=Demars-DMC
COPY $BINARY /usr/bin/Demars-DMC

