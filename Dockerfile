FROM golang:1.17.8 as build

WORKDIR /app

COPY . .

RUN go build

FROM ubuntu:20.04

WORKDIR /app

RUN apt-get update && apt-get install -y ca-certificates && update-ca-certificates

COPY db/migrations ./db/migrations/
COPY --from=build /app/tokenbridge-monitor ./

EXPOSE 3333

ENTRYPOINT ./tokenbridge-monitor
