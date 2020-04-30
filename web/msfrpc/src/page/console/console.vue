<template>
  <el-container class="el-container">
    <!-- left area include console list and control -->
    <el-aside class="el-aside" width="350px">

      <!-- control -->
      ws:
      <el-select v-model="workspace" placeholder="default">
        <el-option
            v-for="item in workspaces"
            :key="item.value"
            :label="item.label"
            :value="item.value">
        </el-option>
      </el-select>
      <el-button @click="create">Create</el-button>

      <!-- show console list -->
      <el-table :data="list" stripe style="width: 100%">
        <el-table-column prop="id" label="ID" width="60"></el-table-column>
        <el-table-column prop="prompt" label="Prompt" width="140"></el-table-column>
        <el-table-column prop="busy" label="Busy"></el-table-column>
      </el-table>
    </el-aside>

    <!-- control selected console -->
    <el-main class="el-main">
      <p>Main</p>
      <p>test-bottom</p>
    </el-main>
  </el-container>
</template>

<script>
import {newLogger} from "../../tool/logger.js"
import fetch from "../../config/fetch"

let logger = newLogger("console")

export default {
  name: "console",

  watch: { "$route": { handler(route) {
    if (route.name === "console") {
      this.getList()
      logger.debug("haha")
    }
  }}},

  data() {
    return {
      workspaces: [
        {value: "default", label: "default"},
        {value: "test", label: "test"}
      ],
      workspace: "default",
      list: []
    }
  },

  mounted() {
    this.getList();
  },

  methods: {
    getList() {
      let pageList = this.list;


      let req = fetch("GET", "/console/list");
      req.then((response) => {
        pageList.length = 0;

        let consoles = response.data["consoles"]
        for (let i = 0; i < consoles.length; i++) {

          pageList.push({
            id: consoles[i].id,
            prompt: consoles[i].prompt,
            busy: consoles[i].busy.toString()
          })
        }

      }).catch(function (error = {}) {
        console.log(error)
      })
    },

    create () {
      let t = this;

      let options = {
        "workspace": "",
        "io_interval": 0
      }
      let req = fetch("POST", "/console/create", options);
      req.then(function (response) {
        console.log(response)
      }).catch(function(error = {}) {
        console.log(error)
      }).finally(function (){
        t.getList();
      })
    },
  },

}
</script>

<style scoped>

</style>