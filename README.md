# 必需修改的常量
创建go-chat-server;
[mysqlConfig] socket
[log] path
config.go 的defaultPath
前端的store
vite.config.js的证书路径
在store里填入iceConfig

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

## TLS 证书 (HTTPS) 配置

项目使用 HTTPS 提供安全的媒体流和 API 调用，需自行准备证书和私钥：

1. 在项目根目录下创建一个 `certs/` 文件夹（已在 `.gitignore` 中忽略，确保不会被提交到仓库）：  
   ```bash
   mkdir certs
2. 将你的证书和私钥文件放到这个目录下，命名示例：
certs/
├── server.crt   # 公钥证书（PEM 格式）
└── server.key   # 私钥（PEM 格式）
（可选）本地开发自签证书快速生成命令：
openssl req -x509 -newkey rsa:4096 \
  -nodes -keyout certs/server.key \
  -out certs/server.crt \
  -days 365 \
  -subj "/CN=localhost"

# 生成 P-256 曲线的私钥
openssl ecparam -genkey -name prime256v1 -out ecdsa.key

# 用私钥创建 CSR
openssl req -new -key ecdsa.key -out ecdsa.csr \
  -subj "/CN=localhost"

# 自签 crt
openssl x509 -req -in ecdsa.csr -signkey ecdsa.key \
  -out ecdsa.crt -days 365

3. 在 config.yml（或代码中）指定证书路径，例如：
tls:
  cert_file: "./certs/server.crt"
  key_file:  "./certs/server.key"