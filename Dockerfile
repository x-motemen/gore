FROM golang:1.14.1-alpine3.11

RUN apk add --no-cache git
RUN go get -u github.com/mdempsky/gocode   # for code completion
RUN go get -u github.com/motemen/gore/cmd/gore

ENTRYPOINT ["gore"]
