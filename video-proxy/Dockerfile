FROM alpine:3.14.1
RUN apk add --no-cache ca-certificates
ARG BINARY
COPY ./$BINARY /sbin/app
ENTRYPOINT ["/sbin/app"]