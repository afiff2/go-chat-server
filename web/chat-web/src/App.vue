<template>
  <router-view />
</template>

<script>
import { watch } from 'vue'
import { useStore } from 'vuex'
import axios from 'axios'
import router from './router'
import { ElMessage } from 'element-plus'

export default {
  name: 'App',
  setup() {
    const store = useStore()

    // 核心初始化函数：获取用户、检查状态、打开 WebSocket
    const initAuth = async (uuid) => {
      if (!uuid) return

      try {
        // 1. 拉取用户信息
        const { data: rsp } = await axios.post(
          `${store.state.backendUrl}/user/get`,
          { uuid }
        )
        if (rsp.code !== 200) {
          console.error(rsp.message)
          return
        }
        const user = rsp.data
        if (!user.avatar.startsWith('http')) {
          user.avatar = store.state.backendUrl + user.avatar
        }
        store.commit('setUserInfo', user)

        // 2. 如果状态码表示被封禁，登出并跳转
        if (user.status === 1) {
          store.commit('cleanUserInfo')
          await axios.post(
            `${store.state.backendUrl}/ws/logout`,
            { owner_id: uuid }
          )
          router.push('/login')
          ElMessage.success('账号被封禁，已退出登录')
          return
        }

        // 3. 建立 WebSocket 连接
        const ws = new WebSocket(
          `${store.state.wsUrl}/ws/login?client_id=${uuid}`
        )
        ws.onopen = () => console.log('WebSocket 连接已打开')
        ws.onmessage = (e) => console.log('收到消息：', e.data)
        ws.onclose = () => console.log('WebSocket 已关闭')
        ws.onerror = (e) => console.error('WebSocket 错误：', e)
        store.commit('setSocket', ws)

      } catch (err) {
        console.error(err)
      }
    }

    // 监视 userInfo.uuid，有值时立即调用 initAuth
    watch(
      () => store.state.userInfo.uuid,
      initAuth,
      { immediate: true }
    )

    return {}
  }
}
</script>