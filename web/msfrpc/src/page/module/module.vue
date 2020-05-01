<template>
  <v-main class="v-main-mt">
    <div class="d-flex flex-row" style="height: 100%">
      <!-- left part -->
      <v-card class="cs-npm" tile min-width="310px" :width="leftPartSize">
        <!-- control components-->
        <v-combobox class="cs-nbr" solo clearable hide-details flat dense :items="directory"
                    label="select a session to use post module"
                    style="margin-bottom: 1px; border: 1px solid"
        ></v-combobox>
        <!-- search module-->
        <v-text-field class="cs-npm cs-nbr" solo-inverted clearable hide-details flat dense
                      label="search module" v-model="search"
        ></v-text-field>
        <!-- module folder -->
        <v-treeview class="cs-npm cs-nbr" transition open-on-click activatable return-object
                    :items="directory" :search="search" :active.sync="selected"
                    style="height: calc(100vh - 150px); overflow-y: auto"
        >
          <template v-slot:prepend="{ item }">
            <v-icon v-if="item.children" v-text="`mdi-folder`"></v-icon>
            <v-icon v-else v-text="`mdi-file`"></v-icon>
          </template>
        </v-treeview>
        <!-- show full path -->
        <v-text-field class="cs-npm cs-nbr" solo-inverted readonly hide-details flat dense
                      label="module full path" v-model="fullPath" @click="selectFullPath"
        ></v-text-field>
      </v-card>
      <v-divider class="pl-1" vertical style="visibility: hidden"></v-divider>
      <!-- right part -->
      <!-- 5px = pl-1(4px) + v-divider(1px) -->
      <v-card class="cs-npm" tile max-width="70%" :min-width="`calc(100% - 5px - ${leftPartSize})`">
        <!-- tabs include "current" and user fixed -->
        <v-tabs height="39px" show-arrows v-model="tab_index" style="border: 1px grey solid">
          <v-tabs-slider color="blue"></v-tabs-slider>
          <v-tab v-for="tab in tab_items" :key="tab.key" style="font-size: 18px">{{tab.tab}}</v-tab>
        </v-tabs>
        <!-- information about user selected -->
        <v-tabs-items v-model="tab_index">
          <v-tab-item v-for="tab in tab_items" :key="tab.key">
            <!-- module name and control tab -->
            <div class="d-flex flex-row" style="height: 100%">
              <!-- module name -->
              <v-text-field readonly solo hide-details flat dense label="module name"
                            v-model="tab.name" style="font-size: 24px"
              ></v-text-field>
              <!-- control button -->
              <v-btn class="control-button" icon color="blue" v-if="tab.key===0" @click="addTab(tab.key)">
                <v-icon>mdi-plus-circle</v-icon>
              </v-btn> <!-- fix -->
              <v-btn class="control-button" icon color="blue" v-if="tab.key!==0" @click="addTab(tab.key)">
                <v-icon>mdi-content-copy</v-icon>
              </v-btn> <!-- copy -->
              <v-btn class="control-button" icon color="red" v-if="tab.key!==0" @click="deleteTab(tab.key)">
                <v-icon>mdi-close-circle</v-icon>
              </v-btn> <!-- delete -->
            </div>

            <!-- module options and console -->
            <div class="d-flex flex-row" style="height: 100%">
              <!-- left part about module information -->
              <v-card class="operation-card" tile :width="infoPartSize">
                <!-- description -->
                <v-textarea readonly hide-details flat label="description" v-model="tab.description"
                ></v-textarea>




              </v-card>
              <v-divider class="pl-1" vertical style="visibility: hidden"></v-divider>
              <!-- right part about console -->
              <v-card class="operation-card" tile :width="`calc(100% - 5px - ${infoPartSize})`">
                <v-textarea  readonly solo hide-details flat background-color="grey"
                            no-resize rows="22"
                            label="console" v-model="m_console"
                            height="100%" class="cs-npm flex-grow-1"
                ></v-textarea>
                <v-btn height="38px" color="blue">Ctrl+C</v-btn>
                <v-btn height="38px" color="blue">Ctrl+Break</v-btn>
                <v-btn height="38px" color="red" width="100px">Destroy</v-btn>
              </v-card>
            </div>
          </v-tab-item>
        </v-tabs-items>
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

function typeToString(type) {
  switch (type) {
    case TYPE_EXPLOIT:
      return "exploit"
    case TYPE_AUXILIARY:
      return "auxiliary"
    case TYPE_POST:
      return "post"
    case TYPE_PAYLOAD:
      return "payload"
    case TYPE_ENCODER:
      return "encoder"
    case TYPE_NOP:
      return "nop"
    case TYPE_EVASION:
      return "evasion"
    default:
      logger.error("unknown type:", type)
  }
}

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
      // about left part
      leftPartSize : "340px",
      count: types.length,  // about module id, v-treeview-node need it
      directory: directory, // about module directory
      loading: false,       // show loading progress bar // TODO need improve performance
      selected: [],         // current selected module
      fullPath: "",         // full path about current selected module
      search: null,         // for search module

      // about right part
      tab_index: 0,
      tab_key: 0, // when create a new tab, it will be added
      tab_items: [
        {
          key: 0,             // tab key for v-for :key
          tab: "current",     // tab name
          name: "",           // module name
          description: "",    // module description
        }
      ],

      // part size about module information
      infoPartSize : "50%",
    }
  },

  mounted() {
    this.getList()
  },

  watch: {
    // get information when select a module
    selected(val) {
      let current = this.tab_items[0]
      // when not select module, clean all data and current tab
      if (val.length === 0) {
        // left part
        this.fullPath = ""
        // right part
        current.name = ""
        current.description = ""
        return
      }
      let module = val[0]
      // update current tab about module information
      let data = {
        type: typeToString(module.type),
        name: module.fullPath
      }
      fetch("POST", "/module/info", data).
      then((resp)=>{
        let data = resp.data
        // left part
        this.fullPath = module.fullPath
        // right part
        current.name = "Name: " + data["name"]
        current.description = data["description"]
      }).catch((err)=>{
        logger.error(`failed to get information about ${data.type} module ${data.name}:`, err)
      })
    }
  },

  methods: {
    // getList is used to get all modules.
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

        // TODO skip exploit modules
        if (types[i].path === "exploits") {
          continue
        }

        fetch("GET", `/module/${types[i].path}`).
        then((resp) => {
          let modules = resp.data["modules"]
          this.addModuleToDirectory(modules, types[i].type)
          // load finished
          if (i === types.length - 1) {
            this.loading = false
          }
        }).catch((err) => {
          logger.error(`failed to get modules about ${types[i].err}:`, err)
        })
      }
    },

    // TODO file under folder
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

    // copy current text to clipboard
    selectFullPath(event) {
      event.target.select()
      try {
        document.execCommand("copy")
      } catch(err) {
        logger.error("failed to copy to clipboard:", err)
      }
    },

    addTab(index) {
      console.log("add: ", index)

    },

    deleteTab(index) {
      console.log("delete: ", index)
    },

  }
}
</script>

<style type="scss" scoped>
.v-tab {
  font-size: 18px;
}

.operation-card {
  @extend .cs-npm;
  height: calc(100vh - 112px);
  overflow-y: auto;
}

.control-button {
  height: 38px;
  width: 38px;
}

/*
  .v-tab-item 75 = system bar (32 px) margin-top (1 px)
  v-tabs height (39px) border (3px).

.v-tab-item {
  height: calc(100vh - 75px);
}
 */
</style>