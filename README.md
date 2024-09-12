# Social-hub

## Запуск


1. git clone
2. docker build . -t social-hub:0.0.7
3. docker network create pgnet
4. docker compose -f cash-compose.yml up -d
5. docker compose -f db-compose.yml up -d
<!-- 6. docker-compose -f db-dialog-compose.yml -p citus up --scale worker=2 -d -->
7. docker compose -f rmq-compose.yml up -d
8. docker compose -f tarantool-compose.yml up -d
8. docker compose -f be-compose.yml up -d
9. в init.sql закомментировано заполнение тестовыми данными
10. в dialog-init.sql инициализация шардированной базы с диалогами


## Тестирование

Коллекция Postman:
    - SocialHub.postman_collection.json
