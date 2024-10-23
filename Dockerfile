FROM alpine:3.20.3
RUN apk add --no-cache ca-certificates git bash
COPY bailiff /
ENTRYPOINT ["/bailiff"]