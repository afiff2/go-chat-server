# 必需修改的常量
[mysqlConfig] socket
[log] path
config.go 的defaultPath
前端的store

# 启动 Zookeeper
bin/zookeeper-server-start.sh config/zookeeper.properties

# 新开终端，启动 Kafka
bin/kafka-server-start.sh config/server.properties


Redis表
user_info_{userId}            //用户信息 ok
contact_mygroup_list_{userId} //我创建的群组 ok
my_joined_group_list_{userId} //我加入的群组 ok
"contact_user_list_" + userId //我的好友（不含群组）列表 ok

group_info_{groupId}        //组的信息 ok
group_memberlist_{groupId}  //组中的人 ok
group_messagelist_{groupId} //组中message

"session_" + userId + "_" + userId/groupId //会话 ok
"session_list_" + userId   //（创建会话人id）会话（人）列表 ok
("group_session_list_" + userId) //（创建会话人id）会话（组）列表 ok

contact_info_  + contactId // 联系人/群信息（类似user_info,gourp_info） (需要没被禁用)！！ok

//跟换topic设置时先手动删除原来的
多个sql操作加入事务（先查状态，再插入 / 删除。并发情况下很容易出现 读到旧快照、重复插入或删错数据。）
在更新前始终读数据库（MySQL），绕过缓存；写操作完成后，再按“先删缓存后删＋延迟删”或“先更新数据库＋再写新缓存”的策略来刷新缓存。

ToDO:
[GIN-debug] [WARNING] Running in "debug" mode. Switch to "release" mode in production.
vue的界面很奇怪