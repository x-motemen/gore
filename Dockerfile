FROM golang:1.24.4-alpine3.22

RUN apk add --no-cache git make \
 && go install golang.org/x/tools/gopls@latest

WORKDIR /go/src/github.com/x-motemen/gore/
COPY go.* ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
RUN make install

ENTRYPOINT ["gore"]
