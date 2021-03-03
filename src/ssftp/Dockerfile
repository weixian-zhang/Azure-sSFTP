FROM golang:1.14.3-alpine AS build_base

RUN apk add --no-cache git

WORKDIR /app

COPY src/. .

WORKDIR /app/ssftp

RUN GOOS=linux

RUN go build -o /app/out

FROM alpine:3.9

COPY --from=build_base /app/out /ssftp

CMD ["/ssftp"]