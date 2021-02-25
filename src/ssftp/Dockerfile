#https://hub.docker.com/r/supinf/go-swagger/dockerfile

FROM golang:1.14.3-alpine AS build_base

RUN apk add --no-cache git

# Set the Current Working Directory inside the container
WORKDIR /app

# We want to populate the module cache based on the go.{mod,sum} files.
# COPY src/go.mod .
# COPY src/go.sum .

#RUN go mod download

COPY src/. .

WORKDIR /app/ssftp

# Build the Go app
RUN GOOS=linux

RUN go build -o /app/out

# start fresh from a smaller image
FROM alpine:3.9

COPY --from=build_base /app/out /ssftp

CMD ["/ssftp"]