# build stage
FROM golang:1.24-alpine AS build-env
RUN apk --no-cache add build-base git gcc

RUN mkdir -p /build
COPY ./ /build/

WORKDIR /build/cmd/ctrl/
RUN go version
RUN go build -o ctrl

# final stage
FROM alpine

RUN apk update && apk add curl && apk add nmap

WORKDIR /app
COPY --from=build-env /build/cmd/ctrl/ctrl /app/

CMD ["ash","-c","/app/ctrl"]
