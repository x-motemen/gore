FROM golang:1.24.0-alpine3.21

RUN apk add --no-cache git make \
 && go install github.com/mdempsky/gocode@latest

WORKDIR /go/src/github.com/x-motemen/gore/
COPY go.* ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
RUN make install

ENTRYPOINT ["gore"]
