FROM golang:1.16-stretch as build

ENV GO111MODULE=on
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64

# config
WORKDIR /go/src/nano-gpu-agent
COPY . .
# RUN GO111MODULE=on go mod download
RUN export CGO_LDFLAGS_ALLOW='-Wl,--unresolved-symbols=ignore-in-object-files' && \
    go build -ldflags="-s -w" -o /go/bin/nano-gpu-agent cmd/main.go
RUN go build -ldflags="-s -w" -o /go/bin/hook           cmd/nano-gpu-hook/main.go

# runtime image
FROM debian:bullseye-slim

ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=utility

COPY --from=build /go/bin/nano-gpu-agent    /usr/bin/nano-gpu-agent
COPY --from=build /go/bin/hook              /usr/bin/hook

COPY tools/nanogpu-nvidia-container-toolkit /usr/bin/nanogpu-nvidia-container-toolkit
COPY tools/install.sh                       /usr/bin/install.sh
COPY tools/mount_nano_gpu                   /usr/bin/mount_nano_gpu

