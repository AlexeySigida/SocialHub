FROM golang:1.22.3

WORKDIR /app

RUN apt-get update && \
    apt-get install libssl-dev -y

RUN go mod init social-hub && \
    go get github.com/golang-jwt/jwt/v4 && \
    go get github.com/google/uuid && \
    go get github.com/lib/pq && \
    go get github.com/go-redis/redis/v8 && \
    go get github.com/gorilla/websocket && \
    go get github.com/streadway/amqp && \
    go get github.com/tarantool/go-tarantool

COPY main.go /app

RUN go build -o social-hub main.go

CMD ["./social-hub"]