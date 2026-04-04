FROM golang:1.25-alpine AS builder

WORKDIR /app

# copy these two files before download for docker caching
COPY go.mod ./
COPY go.sum ./
RUN go mod download
RUN go mod tidy

COPY . .

RUN go build -o cloud-ramp ./cmd/cloud-ramp

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/cloud-ramp .

EXPOSE 8080
CMD ["./cloud-ramp"]