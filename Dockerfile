FROM golang:1.20-bullseye as builder
COPY . /app
WORKDIR /app
RUN go mod download
WORKDIR /app/cmd
RUN go build -o main

FROM ubuntu:jammy as runnner
COPY --from=builder /app/cmd/main /app/cmd/main
COPY --from=builder /app/res/config.yaml /app/res/config.yaml
WORKDIR /app/cmd
CMD ["./main"]