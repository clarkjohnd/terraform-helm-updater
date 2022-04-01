# Build stage
FROM golang:alpine AS builder
RUN apk add --no-cache git
WORKDIR /go/src/app
COPY . .
RUN go get -d -v ./...
RUN go build -o /go/bin/app -v ./...

# Add app to Helm image
FROM alpine/helm:3.8.1
COPY --from=builder /go/bin/app /app
# Install and configure gh cli and permissions to modify files and create a pull request
RUN mkdir /usr/bin/gh && \
    wget https://github.com/cli/cli/releases/download/v2.7.0/gh_2.7.0_linux_amd64.tar.gz -O ghcli.tar.gz && \
    tar --strip-components=1 -xf ghcli.tar.gz -C /usr/bin/gh
ENV PATH="${PATH}:/usr/bin/gh/bin"
ENTRYPOINT /app
LABEL Name=terraform-helm-updater Version=0.1.0
