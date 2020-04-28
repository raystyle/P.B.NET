<template>
  <el-row class="el-row">
    <el-col :span="2">
      <div>icon</div>
    </el-col>
    <el-col :span="2">
      <el-button class="el-button" @click="$router.push('/session')">Session</el-button>
    </el-col>
    <el-col :span="2">
      <el-button class="el-button" @click="$router.push('/job')">Job</el-button>
    </el-col>
    <el-col :span="2">
      <el-button class="el-button" @click="$router.push('/console')">Console</el-button>
    </el-col>
    <el-col :span="2">
      <el-button class="el-button" @click="$router.push('/node')">Host</el-button>
    </el-col>
    <el-col :span="2">
      <el-button class="el-button" @click="$router.push('/node')">Database</el-button>
    </el-col>
    <el-col :span="2">
      <el-button class="el-button" @click="$router.push('/node')">Node</el-button>
    </el-col>
    <el-col :span="2">
      <el-button class="el-button" @click="$router.push('/map')">Map</el-button>
    </el-col>
  </el-row>
</template>

<script>
export default {
  name: "c-header"
}
</script>

<style scoped>
.el-row {
  padding: 2px;
}

</style>
