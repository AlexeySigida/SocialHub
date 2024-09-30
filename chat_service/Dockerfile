FROM golang:1.22.3

WORKDIR /app

RUN wget https://repo.zabbix.com/zabbix/6.4/ubuntu/pool/main/z/zabbix-release/zabbix-release_6.4-1+ubuntu22.04_all.deb
RUN dpkg -i zabbix-release_6.4-1+ubuntu22.04_all.deb
# RUN apt update -y
# RUN apt install zabbix-agent -y

RUN apt-get update && \
    apt-get install libssl-dev zabbix-agent:6.4.0 -y && \
    mkdir -p /var/run/zabbix && \
    mkdir -p /var/run/zabbix && \  
    mkdir -p /var/log/zabbix && \  
    chown -R zabbix:zabbix /var/run/zabbix && \  
    chown -R zabbix:zabbix /var/log/zabbix  


COPY main.go /app
COPY zabbix_agentd.conf /etc/zabbix/zabbix_agentd.conf

RUN go mod init chat-service && \
    go get github.com/gorilla/mux && \
    go get github.com/tarantool/go-tarantool && \
    go get github.com/lib/pq && \
    go get github.com/prometheus/client_golang/prometheus && \
    go get github.com/prometheus/client_golang/prometheus/promhttp

RUN go build -o chat-service main.go

# Expose ports
EXPOSE 8081 10050 10051

CMD ["sh", "-c", "./chat-service & zabbix_agentd -f"]