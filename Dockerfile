FROM golang:1.10
WORKDIR /go/src/go-teemoproject/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o go-teemoproject .

FROM ubuntu:18.04
COPY build /tmp/build
RUN /tmp/build && rm -rf /tmp/build
COPY --from=0 /go/src/go-teemoproject/go-teemoproject /root/
WORKDIR /root/
CMD ["./go-teemoproject", "-logtostderr"]
