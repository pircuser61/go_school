#### Запуск

1. Собрать docker-образ с приложением (buildDockerImage.bat)
   `docker build -t school_app -f app.dockerfile . `
2. Развернуть сервисы
   `docker-compose up`

#### Postman-коллекция

SchoolMaterial.postman_collection.json

- В коллекции объявлены 3 переменных
  - host: по умолчанию 127.0.0.1
  - port: по умолчанию 8080
  - id: при выполнении Create в нее прописывается UUID созданной записи,
    - при выполнии List - UUID первой записи в списке
    - GET,UPDATE,DELETE - используют полученный id
- В сервисе списка материалов, для фильтов, дата должна быть в виде 2024-08-05T03:00:00%2B03:00 (%2B экранирование знака "плюс" для +03:00)
