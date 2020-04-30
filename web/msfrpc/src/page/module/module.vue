<template>
  <v-main class="v-main-mt">
    <div class="d-flex flex-row" style="height: 100%">
      <!-- left part -->
      <v-card class="pa-0 ma-0 d-flex flex-column" tile flat min-width="310px" :width="leftSize">
        <!-- control components-->
        <v-combobox class="cs-nbr" solo clearable hide-details flat dense :items="directory"
                    label="select a session to use post module"
                    style="margin-bottom: 1px; border: 1px solid"
        ></v-combobox>
        <v-card class="pa-0 ma-0" height="100%" tile flat :loading="loading">
          <!-- search module-->
          <v-text-field class="pa-0 ma-0 cs-nbr" solo-inverted clearable hide-details flat dense
                        v-model="search" label="search module"
          ></v-text-field>
          <!-- module folder -->
          <v-treeview class="pa-0 ma-0 cs-nbr" transition open-on-click activatable return-object
                      :items="directory" :search="search" :active.sync="selected">
            <template v-slot:prepend="{ item }">
              <v-icon v-if="item.children" v-text="`mdi-folder`"></v-icon>
              <v-icon v-else v-text="`mdi-file`"></v-icon>
            </template>
          </v-treeview>
        </v-card>
        <!-- show full path -->
        <v-text-field class="pa-0 ma-0 cs-nbr" solo-inverted readonly hide-details flat dense
                      v-model="lastSelected" label="module full path" @click="selectFullPath"
        ></v-text-field>
      </v-card>
      <v-divider class="pl-1" vertical style="visibility: hidden"></v-divider>
      <!-- right part 5px = pl-1(4px) + v-divider(1px) -->
      <v-card class="pa-0 ma-0" tile flat max-width="70%" :min-width="`calc(100% - 5px - ${leftSize})`">
        <v-tabs height="39px" show-arrows style="border: 1px grey solid">
          <v-tabs-slider></v-tabs-slider>
          <v-tab class="v-tab">current</v-tab>
          <v-tab class="v-tab">exploit</v-tab>
          <v-tab class="v-tab"
                 v-for="i in 30"
                 :key="i"
          >
            Item {{ i }}
          </v-tab>
        </v-tabs>
      </v-card>
    </div>
  </v-main>
</template>

<script>
import {newLogger} from "../../tool/logger.js"
import fetch from "../../config/fetch.js"

const TYPE_EXPLOIT = 0
const TYPE_AUXILIARY = 1
const TYPE_POST = 2
const TYPE_PAYLOAD = 3
const TYPE_ENCODER = 4
const TYPE_NOP = 5
const TYPE_EVASION = 6

let logger = newLogger("module")

export default {
  name: "module",

  data() {
    // create module folders
    let directory = []
    let types = ["exploit", "auxiliary", "post",
      "payload", "encoder", "nop", "evasion"]
    for (let i = 0; i < types.length; i++) {
      directory.push({
        id: i,
        name: types[i],
        children: []
      })
    }
    return  {
      leftSize : "340px",
      count: types.length,  // about module id, v-treeview-node need it
      directory: directory, // about module directory
      loading: false,       // show loading progress bar // TODO need improve performance
      selected: [],         // current selected module
      lastSelected: "",     // prevent show the information about same module
      search: null,         // for search module
    }
  },

  mounted() {
    // get module information
    this.$watch("selected", (nv) => {
      if (nv.length === 0) {
        this.lastSelected = ""
        return
      }
      if (this.lastSelected !== nv[0].fullPath) {
        this.lastSelected = nv[0].fullPath
      }
    })
    this.getList()
  },

  methods: {
    getList() {
      let types = [
        {
          path: "exploits",
          type: TYPE_EXPLOIT,
          err: "exploit"
        },
        {
          path: "auxiliary",
          type: TYPE_AUXILIARY,
          err: "auxiliary"
        },
        {
          path: "post",
          type: TYPE_POST,
          err: "post"
        },
        {
          path: "payloads",
          type: TYPE_PAYLOAD,
          err: "payload"
        },
        {
          path: "encoders",
          type: TYPE_ENCODER,
          err: "encoder"
        },
        {
          path: "nops",
          type: TYPE_NOP,
          err: "nop"
        },
        {
          path: "evasion",
          type: TYPE_EVASION,
          err: "evasion"
        },
      ]
      this.loading = false // TODO need improve performance
      // reset counter for module id
      this.count = types.length
      // get modules
      for (let i = 0; i < types.length; i++) {
        this.directory[i].children.length = 0
        fetch("GET", `/module/${types[i].path}`).
        then((resp) => {
          let modules = resp.data["modules"]
          this.addModuleToDirectory(modules, types[i].type)
          // load finished
          if (i === types.length - 1) {
            this.loading = false
          }
        }).catch((err) => {
          logger.error(`failed to get modules about ${types[i].err}`, err)
        })
      }
    },

    addModuleToDirectory(modules, type) {
      let current = this.directory[type]
      for (let i = 0; i < modules.length; i++) {
        let sections = modules[i].split("/")
        let folders = sections.slice(0, sections.length-1)  // ["aix", "local"]
        let name = sections[sections.length-1]  // "xorg_x11_server"
        // create folders if not exist.
        for (let i = 0; i < folders.length; i++) {
          let children = current.children
          let exist = false
          for (let j = 0; j < children.length; j++) {
            if (children[j].name === folders[i]) {
              exist = true
              current = children[j]
              break
            }
          }
          if (!exist) {
            this.count += 1
            let folder = {
              id: this.count,
              name: folders[i],
              children: []
            }
            children.push(folder)
            current = folder
          }
        }
        // add module
        this.count += 1
        current.children.push({
          id: this.count,
          type: type,
          name: name,
          fullPath: modules[i]
        })
        // recover to root directory(exploit, auxiliary...)
        current = this.directory[type]
      }
    },

    selectFullPath(event) {
      event.target.select()
      try {
        document.execCommand("copy")
      } catch(err) {
        logger.error("failed to copy to clipboard:", err)
      }
    },

  }
}
</script>

<style type="scss" scoped>
.v-tab {
  font-size: 18px;
}
</style>