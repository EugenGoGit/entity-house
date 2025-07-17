FROM golang:1.24 AS builder

WORKDIR /usr/src/app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download

COPY ./main.go ./main.go
COPY ./entity_feature /entity_feature
CMD ["go","run","main.go"]
