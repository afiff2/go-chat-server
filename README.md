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
检查有没有 .Delete() 触发软删除
检查一下一些服务有没有关闭 redis

# 启动 Zookeeper
bin/zookeeper-server-start.sh config/zookeeper.properties

# 新开终端，启动 Kafka
bin/kafka-server-start.sh config/server.properties

Kafka中消费者数量应该<=partition数量，最好等于
Broker 数量应 ≥ ReplicationFactor（副本因子）


Redis表
user_info_{userId}            //用户信息
contact_mygroup_list_{userId} //我创建的群组
my_joined_group_list_{userId} //我加入的群组
"contact_user_list_" + userId //我的好友列表
message_list_{userOneId}_{userTwoId} //消息列表
group_messagelist_{groupId} //组中message
group_info_{groupId}        //组的信息
group_memberlist_{groupId}  //组中的人
"session_list_" + ownerId   //（创建会话人id）会话（人）列表
("group_session_list_" + ownerId) //（创建会话人id）会话（组）列表



DeleteGroups里面有两个全量删除处理一下
EnterGroupDirectly里面全量删除处理一下
群成员信息应独立建表 group_members (group_id, user_id)；

