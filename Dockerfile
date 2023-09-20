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
FROM debian:11-slim

WORKDIR /camino-synapse-app-service
COPY --from=builder /camino-synapse-app-service/build/ .
# EXPOSE 5000

ENTRYPOINT [ "./camino-synapse-app-service" ]
