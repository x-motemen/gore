FROM golang:1.16.6-alpine3.14

RUN apk add --no-cache git make
WORKDIR /go/src/github.com/x-motemen/gore/
COPY . .
RUN make install

RUN go get -u github.com/mdempsky/gocode   # for code completion

ENTRYPOINT ["gore"]
