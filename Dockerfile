FROM golang:1.14.1-alpine3.11

RUN apk add --no-cache git make
WORKDIR /go/src/github.com/motemen/gore/
COPY . .
RUN make install

RUN go get -u github.com/mdempsky/gocode   # for code completion

ENTRYPOINT ["gore"]
