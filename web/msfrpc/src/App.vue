<template>
  <div id="app">
    <!-- menu for special pages, see src/router/router.js -->
    <el-container class="el-container" v-if="$route.meta.showMenu">
      <el-header id="header" class="el-header" height="44px"><c-header></c-header></el-header>
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

#header {
  padding: 0;
  margin: 0;
  background-color: #1faf1a;
}
</style>