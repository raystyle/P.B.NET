<template>
  <div id="app">
    <!-- menu for special pages, see src/router/router.js -->
    <el-container class="el-container" v-if="$route.meta.showMenu">
      <el-header class="el-header"><c-header></c-header></el-header>
    </el-container>
    <!-- about vue-router -->
    <keep-alive>
      <router-view v-if="$route.meta.keepAlive"></router-view>
    </keep-alive>
    <router-view v-if="!$route.meta.keepAlive"></router-view>
  </div>
</template>

<script>
export default {
  name: "app"
}
</script>

<style>
@import "style/common.css";

.el-header {
  padding: 0;
  margin: 0;
  background-color: #b3c0d1;
  color: #333;
  text-align: left;
  height: 300px;
  line-height: 10px;
}
</style>