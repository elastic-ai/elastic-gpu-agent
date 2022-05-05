FROM golang:1.17-stretch as build

ENV GO111MODULE=on
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64

# config
WORKDIR /go/src/elastic-gpu-agent
COPY . .
# RUN GO111MODULE=on go mod download
RUN export CGO_LDFLAGS_ALLOW='-Wl,--unresolved-symbols=ignore-in-object-files' && \
    go build -ldflags="-s -w" -o /go/bin/elastic-gpu-agent cmd/main.go
RUN go build -ldflags="-s -w" -o /go/bin/egpu-nvidia-container-runtime-hook cmd/elastic-gpu-hook/main.go

# runtime image
FROM debian:bullseye-slim

ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=utility

COPY --from=build /go/bin/elastic-gpu-agent /usr/bin/elastic-gpu-agent
COPY --from=build /go/bin/egpu-nvidia-container-runtime-hook /usr/bin/egpu-nvidia-container-runtime-hook

COPY tools/egpu-nvidia-container-toolkit /usr/bin/egpu-nvidia-container-toolkit
COPY tools/install.sh /usr/bin/install.sh

