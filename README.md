# Social-hub

## Запуск


1. git clone
2. docker build -t social-hub:0.0.4
3. docker network create pgnet
4. docker compose -f cash-compose.yml up -d
5. docker compose -f db-compose.yml up -d
6. docker compose -f be-compose.yml up -d
6. в init.sql закомментировано заполнение тестовыми данными


## Тестирование

Коллекция Postman:
    - SocialHub.postman_collection.json
