# Build the manager binary
FROM golang:1.13.8 as builder

# Copy in the go src
WORKDIR /go/src/sigs.k8s.io/cluster-api-provider-ibmcloud
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY go.mod ./

# Build
RUN export GO111MODULE=on && go mod tidy && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/manager

# Copy the controller-manager into a thin image
FROM alpine:latest
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*
WORKDIR /
COPY --from=builder /go/src/sigs.k8s.io/cluster-api-provider-ibmcloud/manager .
ENTRYPOINT ["/manager"]
