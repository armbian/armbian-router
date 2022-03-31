FROM golang:alpine AS builder

ADD . /src

WORKDIR /src

RUN apk add --no-cache git && \
	export CGO_ENABLED=0 && \
    go build -o /src/dlrouter

FROM alpine

COPY --from=builder /src/dlrouter /usr/bin/dlrouter

CMD "/usr/bin/dlrouter"