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


[GIN-debug] [WARNING] Running in "debug" mode. Switch to "release" mode in production.

# 启动 Zookeeper
bin/zookeeper-server-start.sh config/zookeeper.properties

# 新开终端，启动 Kafka
bin/kafka-server-start.sh config/server.properties

Kafka中消费者数量应该<=partition数量，最好等于
Broker 数量应 ≥ ReplicationFactor（副本因子）