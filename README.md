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
user_info_{userId}            //用户信息 ok
contact_mygroup_list_{userId} //我创建的群组 ok
my_joined_group_list_{userId} //我加入的群组 ok
"contact_user_list_" + userId //我的好友（不含群组）列表 ok

message_list_{userOneId}_{userTwoId} //消息列表

group_info_{groupId}        //组的信息 ok
group_memberlist_{groupId}  //组中的人 ok
group_messagelist_{groupId} //组中message

"session_" + userId + "_" + userId/groupId //会话 ok
"session_list_" + userId   //（创建会话人id）会话（人）列表 ok
("group_session_list_" + userId) //（创建会话人id）会话（组）列表 ok

contact_info_  + contactId // 联系人/群信息（类似user_info,gourp_info） (需要没被禁用)！！ok


多个sql操作加入事务（先查状态，再插入 / 删除。并发情况下很容易出现 读到旧快照、重复插入或删错数据。）