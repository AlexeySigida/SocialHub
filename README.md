# Social-hub

## Запуск


1. git clone
2. docker build . -t social-hub:0.0.9
3. docker build . -t counter-service:0.0.1
4. docker build ./chat-service/. -t chat-service:0.0.2
5. docker network create pgnet
6. docker compose -f cash-compose.yml up -d
7. docker compose -f db-compose.yml up -d
<!-- 6. docker-compose -f db-dialog-compose.yml -p citus up --scale worker=2 -d -->
8. docker compose -f rmq-compose.yml up -d
9. docker compose -f tarantool-compose.yml up -d
10. docker compose -f monitoring-compose.yml up -d
11. docker compose -f be-compose.yml up -d
12. в init.sql закомментировано заполнение тестовыми данными
13. в dialog-init.sql инициализация шардированной базы с диалогами


## Тестирование

Коллекция Postman:
    - SocialHub.postman_collection.json
