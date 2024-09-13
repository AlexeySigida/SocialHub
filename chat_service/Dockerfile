FROM golang:1.22.3

WORKDIR /app

RUN apt-get update && \
    apt-get install libssl-dev -y

COPY main.go /app

RUN go mod init chat-service && \
    go get github.com/gorilla/mux && \
    go get github.com/tarantool/go-tarantool && \
    go get github.com/lib/pq

RUN go build -o chat-service main.go

CMD ["./chat-service"]