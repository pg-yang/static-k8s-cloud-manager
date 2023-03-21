# syntax=docker/dockerfile:1
FROM golang:1.19 AS builder
WORKDIR /build
COPY . .
RUN go env -w GOPROXY=https://goproxy.cn,direct \
  && CGO_ENABLED=0 go build -o static-k8s-cloud-manager ./main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /k8s
COPY --from=builder /build/static-k8s-cloud-manager ./

ENTRYPOINT /k8s/static-k8s-cloud-manager
CMD ["--cloud-provider=static-cloud"]