FROM golang:1.12 as builder
RUN mkdir /build 
ADD . /build/
WORKDIR /build 
RUN go get -u github.com/gobuffalo/packr/packr
RUN packr clean && packr
RUN go build -o main .
FROM alpine as runtime
RUN adduser -S -D -H -h /app appuser
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
USER appuser
COPY --from=builder /build/main /app/
WORKDIR /app
CMD ["./main"]
EXPOSE 3000