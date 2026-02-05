FROM pogocache/pogocache:1.3.1
RUN apk add --no-cache netcat-openbsd libssl3=3.5.5-r0 libcrypto3=3.5.5-r0
USER nobody
ENTRYPOINT ["/usr/local/bin/pogocache"]
