###
### Stage 0: builder
###
FROM golang:1.23.1-alpine3.20 AS builder
RUN apk update && apk upgrade && apk add build-base
WORKDIR /camino-matrix-app-service

COPY . .
RUN go build -o build/

###
### Stage 1: runtime
###
FROM alpine:3.20

RUN apk add libc6-compat

WORKDIR /camino-matrix-app-service
COPY --from=builder /camino-matrix-app-service/build .
COPY --from=builder /camino-matrix-app-service/migrations ./migrations

ENTRYPOINT [ "./camino-matrix-app-service" ]