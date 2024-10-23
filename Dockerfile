FROM alpine:3.20.3
RUN apk add --no-cache ca-certificates git bash openssh
COPY bailiff /
ENTRYPOINT ["/bailiff"]