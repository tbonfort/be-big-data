FROM golang:bookworm AS builder
RUN apt update && apt -y install libgdal-dev
WORKDIR /build
COPY main.go /build
RUN go mod init be && go mod tidy && go build -o be .
  
FROM debian:bookworm
RUN apt update && apt install -y libgdal32 && rm -rf /var/lib/{apt,dpkg,cache,log}
COPY --from=builder /build/be /be
ENTRYPOINT /be
