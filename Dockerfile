###
### Stage 0: builder
###
FROM golang:1.23.9-alpine AS builder
RUN apk update && apk upgrade && apk add build-base
WORKDIR /camino-matrix-app-service

COPY . .
RUN go build -o build/

###
### Stage 1: runtime
###
FROM alpine:3.21

RUN apk add libc6-compat

WORKDIR /camino-matrix-app-service
COPY --from=builder /camino-matrix-app-service/build .

ENTRYPOINT [ "./camino-matrix-app-service" ]