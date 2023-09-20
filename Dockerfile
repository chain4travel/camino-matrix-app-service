###
### Stage 0: builder
###
FROM golang:1.19.6-buster AS builder
RUN apt-get update -qq
WORKDIR /camino-synapse-app-service

COPY . .
RUN go build -o build/

###
### Stage 1: runtime
###
FROM alpine:3.16

RUN apk add libc6-compat

WORKDIR /camino-synapse-app-service
COPY --from=builder /camino-synapse-app-service/build/ .

ENTRYPOINT [ "./camino-synapse-app-service" ]