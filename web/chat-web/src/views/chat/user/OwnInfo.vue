<template>
  <div class="chat-wrap">
    <div class="chat-window" :style="{
      boxShadow: `var(${'--el-box-shadow-dark'})`,
    }">
      <el-container class="chat-window-container">
        <el-aside class="aside-container">
          <NavigationModal></NavigationModal>
          <ContactListModal></ContactListModal>
        </el-aside>
        <div class="owner-info-window">
          <div class="my-homepage-title">
            <h2>我的主页</h2>
          </div>

          <p class="owner-prefix">用户id：{{ userInfo.uuid }}</p>
          <p class="owner-prefix">昵称：{{ userInfo.nickname }}</p>
          <p class="owner-prefix">电话：{{ userInfo.telephone }}</p>
          <p class="owner-prefix">邮箱：{{ userInfo.email }}</p>
          <p class="owner-prefix">
            性别：{{ userInfo.gender === 0 ? "男" : "女" }}
          </p>
          <p class="owner-prefix">生日：{{ userInfo.birthday }}</p>
          <p class="owner-prefix">个性签名：{{ userInfo.signature }}</p>
          <p class="owner-prefix">
            加入chat server的时间：{{ userInfo.created_at }}
          </p>
          <div class="owner-opt">
            <p class="owner-prefix">头像：</p>
            <img style="width: 40px; height: 40px" :src="userInfo.avatar" />
          </div>
        </div>
        <div class="edit-window">
          <el-button class="edit-btn" @click="showMyInfoModal">编辑</el-button>
        </div>
        <Modal :isVisible="isMyInfoModalVisible">
          <template v-slot:header>
            <div class="modal-header">
              <div class="modal-quit-btn-container">
                <button class="modal-quit-btn" @click="quitMyInfoModal">
                  <el-icon>
                    <Close />
                  </el-icon>
                </button>
              </div>
              <div class="modal-header-title">
                <h3>修改主页</h3>
              </div>
            </div>
          </template>
          <template v-slot:body>
            <el-scrollbar max-height="300px" style="
                width: 400px;
                display: flex;
                align-items: center;
                justify-content: center;
                margin-top: 20px;
              ">
              <div class="modal-body">
                <el-form ref="formRef" :model="updateInfo" label-width="70px">
                  <el-form-item prop="nickname" label="昵称" :rules="[
                    {
                      min: 3,
                      max: 10,
                      message: '昵称长度在 3 到 10 个字符',
                      trigger: 'blur',
                    },
                  ]">
                    <el-input v-model="updateInfo.nickname" placeholder="选填" />
                  </el-form-item>
                  <el-form-item prop="email" label="邮箱">
                    <el-input v-model="updateInfo.email" placeholder="选填" />
                  </el-form-item>
                  <el-form-item prop="birthday" label="生日">
                    <el-input v-model="updateInfo.birthday" placeholder="选填，格式为2024.1.1" />
                  </el-form-item>
                  <el-form-item prop="signature" label="个性签名">
                    <el-input v-model="updateInfo.signature" placeholder="选填" />
                  </el-form-item>
                  <el-form-item prop="avatar" label="头像">
                    <el-upload v-model:file-list="fileList" ref="uploadRef" :auto-upload="false" :action="uploadPath"
                      :on-success="handleUploadSuccess" :before-upload="beforeFileUpload">
                      <template #trigger>
                        <el-button style="background-color: rgb(252, 210.9, 210.9)">上传图片</el-button>
                      </template>
                    </el-upload>
                  </el-form-item>
                </el-form>
              </div>
            </el-scrollbar>
          </template>
          <template v-slot:footer>
            <div class="modal-footer">
              <el-button class="modal-close-btn" @click="closeMyInfoModal">
                完成
              </el-button>
            </div>
          </template>
        </Modal>
      </el-container>
    </div>
  </div>
</template>

<script>
import { ref, reactive, toRefs, computed } from "vue"
import { useStore } from "vuex"
import { useRouter } from "vue-router"
import axios from "axios"
import { ElMessage } from "element-plus"
import { Close } from "@element-plus/icons-vue"
import Modal from "@/components/Modal.vue"
import NavigationModal from "@/components/NavigationModal.vue"
import ContactListModal from "@/components/ContactListModal.vue"
import { checkEmailValid } from "@/assets/js/valid.js"

export default {
  name: "OwnInfo",
  components: { Modal, NavigationModal, ContactListModal, Close },
  setup() {
    const router = useRouter()
    const store = useStore()

    // 只读地拿到全局 userInfo
    const userInfo = computed(() => store.state.userInfo)

    // 本地可变状态
    const uploadRef = ref(null)
    const state = reactive({
      updateInfo: {
        uuid: "",
        nickname: "",
        email: "",
        birthday: "",
        signature: "",
        avatar: "",
      },
      isMyInfoModalVisible: false,
      uploadPath: store.state.backendUrl + "/message/upload-avatar",
      fileList: [],
    })

    // 打开弹窗前，把最新的 userInfo 填进去
    function showMyInfoModal() {
      Object.assign(state.updateInfo, {
        uuid: userInfo.value.uuid,
        nickname: userInfo.value.nickname,
        email: userInfo.value.email,
        birthday: userInfo.value.birthday,
        signature: userInfo.value.signature,
        avatar: ""
      })
      state.isMyInfoModalVisible = true
    }

    // 提交修改
    async function closeMyInfoModal() {
      const { nickname, email, birthday, signature } = state.updateInfo
      // 校验至少改一项
      if (!nickname && !email && !birthday && !signature && state.fileList.length === 0) {
        ElMessage.warning("请至少修改一项")
        return
      }
      // 昵称长度校验
      if (nickname && (nickname.length < 3 || nickname.length > 10)) {
        ElMessage.error("昵称长度应在3到10字符之间")
        return
      }
      // 邮箱校验
      if (email && !checkEmailValid(email)) {
        ElMessage.error("请输入有效邮箱")
        return
      }

      // 准备提交的数据
      const payload = { ...state.updateInfo }

      // 如果选了新头像，先触发上传
      if (state.fileList.length > 0) {
        payload.avatar = store.state.avatarPath + state.fileList[0].name
        uploadRef.value.submit()
      }

      // 调用接口
      try {
        const rsp = await axios.post(
          `${store.state.backendUrl}/user/update`,
          payload
        )
        if (rsp.data.code === 200) {
          // 通过 mutation 更新全局 userInfo
          store.commit("setUserInfo", { ...userInfo.value, ...payload })
          ElMessage.success(rsp.data.message)
          state.isMyInfoModalVisible = false
          state.fileList = []
        } else {
          ElMessage.error(rsp.data.message)
        }
      } catch (err) {
        console.error(err)
        ElMessage.error("更新失败")
      }
    }

    function quitMyInfoModal() {
      state.isMyInfoModalVisible = false
      state.fileList = []
    }

    function handleUploadSuccess() {
      ElMessage.success("头像上传成功")
      state.fileList = []
      router.go(0)
    }
    function beforeFileUpload(file) {
      if (state.fileList.length > 1) {
        ElMessage.error("只能上传一张头像")
        return false
      }
      const isLt50M = file.size / 1024 / 1024 < 50
      if (!isLt50M) {
        ElMessage.error("头像大小不能超过50MB")
        return false
      }
    }

    return {
      userInfo,
      uploadRef,
      ...toRefs(state),
      showMyInfoModal,
      closeMyInfoModal,
      quitMyInfoModal,
      handleUploadSuccess,
      beforeFileUpload,
    }
  },
}
</script>

<style scoped>
.owner-info-window {
  width: 84%;
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.owner-prefix {
  font-family: Arial, Helvetica, sans-serif;
  margin: 6px;
}

.owner-opt {
  margin: 6px;
  display: flex;
  flex-direction: row;
}

.edit-window {
  width: 10%;
  display: flex;
  flex-direction: column-reverse;
}

h3 {
  font-family: Arial, Helvetica, sans-serif;
  color: rgb(69, 69, 68);
}

.modal-quit-btn-container {
  height: 30%;
  width: 100%;
  display: flex;
  flex-direction: row-reverse;
}

.modal-quit-btn {
  background-color: rgba(255, 255, 255, 0);
  color: rgb(229, 25, 25);
  padding: 15px;
  border: none;
  cursor: pointer;
  position: fixed;
  justify-content: center;
  align-items: center;
}

.modal-header {
  height: 20%;
  width: 100%;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
}

.modal-body {
  height: 100%;
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.modal-footer {
  height: 20%;
  width: 100%;
  display: flex;
  justify-content: center;
  align-items: center;
}

.modal-header-title {
  height: 70%;
  width: 100%;
  display: flex;
  justify-content: center;
  align-items: center;
}

h2 {
  margin-bottom: 20px;
  font-family: Arial, Helvetica, sans-serif;
}

.el-menu {
  background-color: rgb(252, 210.9, 210.9);
  width: 101%;
}

.el-menu-item {
  background-color: rgb(255, 255, 255);
  height: 45px;
}
</style>