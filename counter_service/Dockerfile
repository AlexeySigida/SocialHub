FROM golang:1.22.3

WORKDIR /counter-service

RUN apt-get update && \
    apt-get install libssl-dev -y

RUN go mod init counter-service && \
    go get github.com/golang-jwt/jwt/v4 && \
    go get github.com/google/uuid && \
    go get github.com/lib/pq && \
    go get github.com/go-redis/redis/v8 && \
    go get github.com/gorilla/websocket && \
    go get github.com/streadway/amqp && \
    go get github.com/tarantool/go-tarantool && \
    go get github.com/gorilla/mux

COPY . /counter-service

RUN go build -o counter-service ./cmd/main.go

CMD ["./counter-service"]