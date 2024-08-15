# Social-hub

## Запуск


1. git clone
2. docker build -t social-hub:0.0.5
3. docker network create pgnet
4. docker compose -f cash-compose.yml up -d
5. docker compose -f db-compose.yml up -d
6. docker-compose -p citus up --scale worker=2 -d
7. docker compose -f be-compose.yml up -d
8. в init.sql закомментировано заполнение тестовыми данными
9. в dialog-init.sql инициализация шардированной базы с диалогами


## Тестирование

Коллекция Postman:
    - SocialHub.postman_collection.json
