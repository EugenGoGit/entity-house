FROM golang:1.24 AS builder

WORKDIR /usr/src/app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download

COPY ./comment ./comment
COPY ./generator ./generator
COPY ./printer ./printer
COPY ./template ./template
COPY ./comment ./comment
COPY ./util ./util
COPY ./main.go ./main.go
RUN go build -v -o /usr/local/bin/app/ ./...

FROM alpine
COPY --from=builder /usr/local/bin/app /usr/local/bin/app
COPY ./impl /impl
COPY ./proto_deps /proto_deps
CMD ["/usr/local/bin/app/entity-house"]
