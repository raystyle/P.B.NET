<template>
  <v-app id="app">
    <!-- menu for special pages, see src/router/router.js -->
    <v-system-bar window v-if="$route.meta.showMenu"><c-menu></c-menu></v-system-bar>
    <!-- about vue-router -->
    <keep-alive>
      <router-view v-if="$route.meta.keepAlive"></router-view>
    </keep-alive>
    <router-view v-if="!$route.meta.keepAlive"></router-view>
  </v-app>
</template>

<script>
export default {
  name: "app"
}
</script>

<style lang="scss">
@import "style/common";

.v-system-bar {
  padding: 0;
  margin: 0;
}
</style>