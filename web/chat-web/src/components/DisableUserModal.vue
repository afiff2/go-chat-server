<template>
  <div style="height: 100%; width: 100%" v-if="isVisible">
    <el-table :data="disableUserTableData" style="width: 100%; height: 90%" @selection-change="selectUsers">
      <el-table-column type="selection" width="55" />
      <el-table-column prop="uuid" label="Uuid" width="200" />
      <el-table-column prop="nickname" label="昵称" width="120" show-overflow-tooltip />
      <el-table-column prop="telephone" label="电话" width="120" show-overflow-tooltip />
      <el-table-column prop="is_admin" label="管理员" width="80">
        <template #default="scope">
          <el-button type="default" v-if="scope.row.is_admin == 0">否</el-button>
          <el-button type="primary" v-if="scope.row.is_admin == 1">是</el-button>
        </template>
      </el-table-column>
      <el-table-column label="删除状态" width="90">
        <template #default="scope">
          <el-button type="default" v-if="scope.row.is_deleted == false">正常</el-button>
          <el-button type="danger" v-if="scope.row.is_deleted == true">已删除</el-button>
        </template>
      </el-table-column>
      <el-table-column label="禁用状态" width="90">
        <template #default="scope">
          <el-button type="default" v-if="scope.row.status == 0">正常</el-button>
          <el-button type="success" v-if="scope.row.status == 1">禁用</el-button>
        </template>
      </el-table-column>
    </el-table>
    <div class="main-able-button-container">
      <el-button style="background-color: rgb(252, 210.9, 210.9); margin-left: 20px"
        @click="updateUsersStatus('disable')">禁用/全部禁用</el-button>
      <el-button style="background-color: rgb(252, 210.9, 210.9)" @click="updateUsersStatus('enable')">启用/全部启用</el-button>
    </div>
  </div>
</template>

<script>
import { ElTable } from "element-plus";
import { watch, reactive, toRefs } from "vue";
import { useStore } from "vuex";
import axios from "axios";
import { ElMessage } from 'element-plus'
export default {
  name: "DisableUserModal",
  props: {
    isVisible: {
      type: Boolean,
      default: false
    }
  },
  setup(props) {
    const store = useStore();
    const data = reactive({
      disableUserTableData: [],
      uuidList: [],
    });
    watch(
      () => props.isVisible,
      visible => { if (visible) getUserList() },
      { immediate: false }
    );
    const getUserList = async () => {
      try {
        const req = {
          owner_id: store.state.userInfo.uuid,
        }
        const rsp = await axios.post(
          store.state.backendUrl + "/user/list", req
        );
        data.disableUserTableData = rsp.data.data;
        console.log(rsp);
      } catch (error) {
        console.log(error);
      }
    };

    const selectUsers = (val) => {
      data.uuidList = val.map((item) => item.uuid);
      console.log(data.uuidList);
    };

    const updateUsersStatus = async (action) => {
      if (!data.uuidList.length) {
        ElMessage.warning('请先选择用户')
        return
      }
      try {
        await axios.post(
          `${store.state.backendUrl}/user/${action}`,
          { uuid_list: data.uuidList }
        )
        ElMessage.success(action === 'ableUsers' ? '已启用' : '已禁用')
        getUserList()
      } catch (e) {
        ElMessage.error(e.message || '操作失败')
      }
    }

    return {
      ...toRefs(data),
      getUserList,
      selectUsers,
      updateUsersStatus,
    };
  },
};
</script>

<style scoped>
.main-able-button-container {
  height: 10%;
  display: flex;
  flex-direction: row-reverse;
  align-items: center;
}
</style>