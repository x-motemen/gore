FROM golang:1.18.0-alpine3.14

RUN apk add --no-cache git make
WORKDIR /go/src/github.com/x-motemen/gore/
COPY . .
RUN make install

RUN go install github.com/mdempsky/gocode@latest   # for code completion

ENTRYPOINT ["gore"]
