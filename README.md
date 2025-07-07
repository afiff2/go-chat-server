# Go‑Chat‑Server

高仿微信的即时通讯 (IM) 系统，采用 **前后端分离** 架构，支持单聊 / 群聊、离线消息、音视频通话及后台管理。

---

## 目录

1. [快速开始](#快速开始)
2. [必需修改的常量](#必需修改的常量)
3. [前端 ](#前端-storeindexjs-模板)[`store/index.js`](#前端-storeindexjs-模板)[ 模板](#前端-storeindexjs-模板)
4. [Redis Key 设计](#redis-key-设计)
5. [TLS 证书 (HTTPS) 配置](#tls-证书-https-配置)
6. [常见问题](#常见问题)

---

## 快速开始

### 环境要求

| 组件  | 说明  |
| ----- | ------- |
| Go    | 后端核心    |
| Node  | 打包前端    |
| MySQL | 关系型数据库  |
| Redis | 缓存   |
| Kafka | 消息队列 & 削峰    |

### 数据库初始化
在启动项目前，请确保已安装并运行了 MySQL 数据库，并手动创建以下数据库：
```mysql
CREATE DATABASE `go-chat-server` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

---

## 必需修改的常量

### 后端

| 文件                   | 字段 / 变量              | 说明                       |
| -------------------- | -------------------- | ------------------------ |
| `config.yml`         | `mysqlConfig.socket` | MySQL **UNIX Socket** 路径 |
|                      | `log.path`           | 后端日志输出绝对路径               |
| `internal/config.go` | `defaultPath`        | configs/config.toml的绝对路径     |


### 前端
修改/web/chat-web/vite.config.js中 HTTPS 证书路径：

---

## 前端 `store/index.js` 模板

> 路径：`./web/chat-web/src/store/index.js`

```javascript
import { createStore } from 'vuex'

export default createStore({
  state: {
    // web服务器地址
    backendUrl: 'https://your_server_ip:8080',
    wsUrl: 'wss://your_server_ip:8080',
    avatarPath: '/static/avatars/',
    userInfo: (sessionStorage.getItem('userInfo') && JSON.parse(sessionStorage.getItem('userInfo'))) || {},
    socket: null,
    iceConfig: {
      iceServers: [
        { urls: "stun:stun.l.google.com:19302" },
        // 自己的 STUN 服务
        { urls: "stun:your_turn_ip:3478" },
        // TURN UDP
        {
          urls: 'turn:your_turn_ip:3478?transport=udp',
          username: 'your_turn_username',
          credential: 'your_turn_credential'
        },
        // TURN TCP
        {
          urls: 'turn:your_turn_ip:3478?transport=tcp',
          username: 'your_turn_username',
          credential: 'your_turn_credential'
        },
      ]
    },
  },
  getters: {
  },
  mutations: {
    setUserInfo(state, userInfo) {
      state.userInfo = userInfo
      sessionStorage.setItem('userInfo', JSON.stringify(userInfo))
    },
    cleanUserInfo(state) {
      state.userInfo = {}
      sessionStorage.removeItem('userInfo')
    },
    setSocket(state, socket) {
      state.socket = socket
    }
  },
  actions: {
  },
  modules: {
  }
})

```

---

## Redis Key 设计

| Key Pattern                     | 作用              |            
| ------------------------------- | --------------- |
| `user_info_{userId}`            | 用户信息            |            
| `contact_mygroup_list_{userId}` | 我创建的群组          |            
| `my_joined_group_list_{userId}` | 我加入的群组          |            
| `contact_user_list_{userId}`    | 我的好友（不含群组）列表    |            
| `group_info_{groupId}`          | 群信息             |            
| `group_memberlist_{groupId}`    | 群成员列表           |            
| `group_messagelist_{groupId}`   | 群消息列表           |            
| `session_{userId}_{userId or groupId}\`| 单人 / 群会话数据 |
| `session_list_{userId}`         | 我的单人会话列表        |            
| `group_session_list_{userId}`   | 我的群会话列表         |            
| `contact_info_{contactId}`      | 联系人 / 群信息|            

---

## TLS 证书 (HTTPS) 配置

> **生产环境强烈推荐使用 Let’s Encrypt 或商用证书**

1. 在项目根目录创建 `certs/` （已加入 `.gitignore`）：

```bash
mkdir certs
```

2. 复制 / 生成证书与私钥到该目录，示例：

```bash
certs/
├── server.crt   # 公钥 (PEM)
└── server.key   # 私钥 (PEM)
```

```bash
# RSA‑4096 自签 (一年有效)
openssl req -x509 -newkey rsa:4096 \
  -nodes -keyout certs/server.key \
  -out   certs/server.crt \
  -days 365 \
  -subj "/CN=localhost"

# P‑256 曲线自签示例
openssl ecparam -genkey -name prime256v1 -out ecdsa.key
openssl req -new -key ecdsa.key -out ecdsa.csr -subj "/CN=localhost"
openssl x509 -req -in ecdsa.csr -signkey ecdsa.key -out ecdsa.crt -days 365
```
