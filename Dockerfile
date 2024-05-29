FROM golang:1.22.3

WORKDIR /app

RUN go mod init social-hub && \
    go get github.com/golang-jwt/jwt/v4 && \
    go get github.com/google/uuid && \
    go get github.com/lib/pq

COPY main.go /app

RUN go build -o social-hub main.go

CMD ["./social-hub"]