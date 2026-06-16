ARG GOLANG_IMAGE=golang:1.26.4-alpine3.24

FROM --platform=$BUILDPLATFORM $GOLANG_IMAGE AS builder

RUN apk add --no-cache git make

WORKDIR /go/src/github.com/x-motemen/gore/
COPY go.* ./
RUN go mod download
COPY . .

ARG TARGETOS TARGETARCH
ENV CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH
RUN make install && go install golang.org/x/tools/gopls@latest \
 && find /go/bin -name gore -o -name gopls | xargs -I% mv % /usr/local/bin/

FROM $GOLANG_IMAGE

RUN apk add --no-cache git
COPY --from=builder /usr/local/bin/gore /usr/local/bin/gopls /usr/local/bin/

ENTRYPOINT ["gore"]
