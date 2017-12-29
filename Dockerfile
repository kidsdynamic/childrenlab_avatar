FROM golang:1.9
RUN mkdir -p /go/src/github.com/kidsdynamic/childrenlab_avatar
ADD build /go/src/github.com/kidsdynamic/childrenlab_avatar/
WORKDIR /go/src/github.com/kidsdynamic/childrenlab_avatar
CMD ["/go/src/github.com/kidsdynamic/childrenlab_avatar/main"]

EXPOSE 8112
