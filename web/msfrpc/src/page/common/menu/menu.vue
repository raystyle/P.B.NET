<template>
  <v-main class="pa-1">
    <div class="d-flex">
      <v-btn class="v-btn" text disabled>MSFRPC</v-btn>
      <v-btn class="v-btn" text to="/session"><i class="mdi mdi-bullseye-arrow"></i>Session</v-btn>
      <v-btn class="v-btn" text to="/job"><i class="mdi mdi-view-headline"></i>Job</v-btn>
      <v-btn class="v-btn" text to="/console"><i class="mdi mdi-console"></i>Console</v-btn>
      <v-btn id="module" class="v-btn" text to="/module"><i class="mdi mdi-view-module"></i>Module</v-btn>
      <v-btn class="v-btn" text to="/node"><i class="mdi mdi-server"></i>Host</v-btn>
      <v-btn class="v-btn" text to="/database"><i class="mdi mdi-database"></i>Database</v-btn>
      <v-btn class="v-btn" text to="/test"><i class="mdi mdi-database"></i>Test</v-btn>
      <div class="mr-auto"></div>
      <v-chip label outlined>{{time}}</v-chip>
      <v-btn class="v-btn" text><i class="mdi mdi-account-box"></i>Admin</v-btn>
    </div>
  </v-main>
</template>

<script>
import {formatDate} from "public/time/time"

export default {
  name: "c-menu",

  data() {
    return {
      time: formatDate((new Date()))
    }
  },

  mounted() {
    this.timer = setInterval(() => {
      this.time = formatDate((new Date()))
    }, 1000)
  },

  beforeDestroy() {
    clearInterval(this.timer)
  }
}
</script>

<style lang="scss" scoped>
.v-btn {
  max-height: 32px;
  font-size: 17px;
}

// fix icon location
#module {
  padding-top: 1px;
}
</style>