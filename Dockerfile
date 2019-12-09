FROM golang:latest as build

WORKDIR /app

ADD . /app/
RUN go build -o main .

FROM alpine
COPY --from=build /app/main /usr/bin/secrets
