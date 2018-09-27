FROM alpine:3.7

RUN apk update && \
    apk upgrade && \
    apk --no-cache add curl jq file

VOLUME [ /demars ]
WORKDIR /demars
EXPOSE 26656 26657
ENTRYPOINT ["/usr/bin/wrapper.sh"]
CMD ["node", "--proxy_app", "dmccoin"]
STOPSIGNAL SIGTERM

COPY wrapper.sh /usr/bin/wrapper.sh
