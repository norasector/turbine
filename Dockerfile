FROM golang:1.17

RUN apt-get update && apt-get install -y libopus-dev libopusfile-dev libhackrf-dev librtlsdr-dev

WORKDIR /app

COPY ./ .

RUN go mod download && go build -o bin/turbine ./cmd/turbine

FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y libopus-dev libopusfile-dev libhackrf-dev librtlsdr-dev

WORKDIR /app

COPY --from=0 /app/bin/turbine ./

EXPOSE 8642

CMD ["/app/turbine"]