FROM golang:1.16 as build

WORKDIR /app

COPY . .

RUN go build

FROM ubuntu:20.10

WORKDIR /app

RUN apt-get update && apt-get install -y ca-certificates && update-ca-certificates

COPY db/migrations ./db/migrations/
COPY --from=build /app/amb-monitor ./

ENTRYPOINT ./amb-monitor
