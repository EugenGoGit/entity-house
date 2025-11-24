FROM golang:1.24 AS builder

WORKDIR /usr/src/app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download

COPY ./internal ./internal
COPY ./cmd/main.go ./cmd/main.go
RUN go build -v -o /usr/local/bin/app/ ./...

FROM alpine
COPY --from=builder /usr/local/bin/app /usr/local/bin/app
COPY ./api_spec_tmpl /api_spec_tmpl
COPY ./proto_deps /proto_deps
COPY ./entity_feature /entity_feature
CMD ["/usr/local/bin/app/cmd"]
