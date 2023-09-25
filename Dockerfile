###
### Stage 0: builder
###
FROM golang:1.19.13-alpine3.18 AS builder
RUN apk update && apk upgrade && apk add build-base
WORKDIR /camino-synapse-app-service

COPY . .
RUN go build -o build/

###
### Stage 1: runtime
###
FROM alpine:3.18

RUN apk add libc6-compat

WORKDIR /camino-synapse-app-service
COPY --from=builder /camino-synapse-app-service/build .
COPY --from=builder /camino-synapse-app-service/migrations ./migrations

ENTRYPOINT [ "./camino-synapse-app-service" ]