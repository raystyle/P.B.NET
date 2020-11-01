<template>
  <v-main>
    <div class="d-flex">
      <v-btn class="v-btn" text disabled>MSFRPC</v-btn>
      <v-btn class="v-btn" text to="/session">
        <div class="pr-2"><i class="mdi mdi-bullseye-arrow"></i>Session</div>
      </v-btn>
      <v-btn class="v-btn" text to="/job">
        <div class="pr-2"><i class="mdi mdi-view-headline"></i>Job</div>
      </v-btn>
      <v-btn class="v-btn" text to="/console">
        <div class="pr-2"><i class="mdi mdi-console"></i>Console</div>
      </v-btn>
      <v-btn id="module" class="v-btn" text to="/module">
        <div class="pr-2"><i class="mdi mdi-view-module"></i>Module</div>
      </v-btn>
      <v-btn class="v-btn" text to="/node">
        <div class="pr-2"><i class="mdi mdi-server"></i>Host</div>
      </v-btn>
      <v-btn class="v-btn" text to="/database">
        <div class="pr-2"><i class="mdi mdi-database"></i>Database</div>
      </v-btn>
      <v-btn class="v-btn" text to="/test">
        <div class="pr-2"><i class="mdi mdi-database"></i>Test</div>
      </v-btn>
      <div class="mr-auto"></div>
      <v-chip label outlined>{{time}}</v-chip>
      <v-btn class="v-btn" text>
        <div class="pr-1"><i class="mdi mdi-account-box"></i>Admin</div>
      </v-btn>
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
  font-size:  17px;
}

// fix icon location
#module {
  padding-top: 1px;
}
</style>