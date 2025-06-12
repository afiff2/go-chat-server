curl -X POST http://localhost:8080/user/register \
     -H "Content-Type: application/json" \
     -d '{
           "telephone": "13800000000",
           "password": "123456",
           "nickname": "testuser"
         }'


curl -X POST http://localhost:8080/user/login \
     -H "Content-Type: application/json" \
     -d '{
           "telephone": "13800000000",
           "password": "123456"
         }'

curl -X POST http://localhost:8080/user/delete \
     -H "Content-Type: application/json" \
     -d '{
           "uuid_list": ["uuid1", "uuid2"]
         }'