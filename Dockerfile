FROM golang:1.19.4-alpine3.17

RUN apk add --no-cache git make
WORKDIR /go/src/github.com/x-motemen/gore/
COPY . .
RUN make install

RUN go install github.com/mdempsky/gocode@latest   # for code completion

ENTRYPOINT ["gore"]
