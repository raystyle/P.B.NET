<template>
  <v-main class="v-main-mt">
    <div class="flex" style="height: 100%">
      <!-- top part about session list-->
      <v-card class="cs-npm" tile>
        <v-data-table
            dense show-select
            :headers="headers" :items="sessions" :items-per-page="10">
        </v-data-table>
      </v-card>
      <!-- divider between top and bottom-->
      <v-divider class="pt-1" style="visibility: hidden"></v-divider>
      <!-- bottom part about session operation-->
      <v-card class="cs-npm" tile>
        acg

      </v-card>
    </div>
  </v-main>
</template>

<script>
import {newLogger} from "../../tool/logger.js"
import {
  getSessionList,
} from "../../api/session"

let logger = newLogger("console")

export default {
  name: "session",

  watch: { "$route": { handler(route) {
    if (route.name === "session") {
      this.refreshSessionList()
    }
  }}},

  data() {
    return {
      headers: [
        {text: "ID", value: "id"},
        {text: "Host", value: "session_host"},
        {text: "Port", value: "session_port"},
        {text: "Tunnel Local", value: "tunnel_local"},
        {text: "Tunnel Peer", value: "tunnel_peer"},
        {text: "Routes", value: "routes"},
        {text: "Type", value: "type"},
        {text: "Platform", value: "platform"},
        {text: "Architecture", value: "architecture"},
        {text: "Username", value: "username"},
        {text: "Information", value: "information"},
        {text: "Workspace", value: "workspace"},
      ],
      sessions: [],
    }
  },

  mounted() {
    this.refreshSessionList();
  },

  methods: {
    refreshSessionList() {
      getSessionList(true).then((resp) => {
        let sessionList = []
        let sessions = resp.data["sessions"]
        let sidList = Object.keys(sessions)

        for (let i = 0; i < sidList.length; i++){
          let sid = sidList[i]
          let session = sessions[sid]

          session.id = sid
          if (session.routes === "") {
            session.routes = "null"
          }

          sessionList.push(session)
        }
        this.sessions = sessionList
      }).catch((err) => {
        logger.error(`failed to get session list:`, err)
      })
    },
  },
}
</script>

<style type="scss" scoped>

</style>