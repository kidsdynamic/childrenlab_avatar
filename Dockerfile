FROM golang:1.8.3
RUN mkdir -p /go/src/github.com/kidsdynamic/childrenlab_avatar
ADD . /go/src/github.com/kidsdynamic/childrenlab_avatar/
WORKDIR /go/src/github.com/kidsdynamic/childrenlab_avatar
RUN go build -o main .
CMD ["/go/src/github.com/kidsdynamic/childrenlab_avatar/main"]

EXPOSE 8112
