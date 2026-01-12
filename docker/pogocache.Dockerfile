FROM pogocache/pogocache:1.3.0
RUN apk add --no-cache netcat-openbsd
USER nobody
ENTRYPOINT ["/usr/local/bin/pogocache"]
