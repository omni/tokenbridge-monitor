FROM golang:1.17.8 as build

WORKDIR /app

COPY . .

RUN mkdir out && go build -o ./out ./cmd/...

FROM ubuntu:20.04

WORKDIR /app

RUN apt-get update && apt-get install -y ca-certificates && update-ca-certificates

COPY db/migrations ./db/migrations/
COPY --from=build /app/out/monitor ./

EXPOSE 3333

ENTRYPOINT ./monitor
