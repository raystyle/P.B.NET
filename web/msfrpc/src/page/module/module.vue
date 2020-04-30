<template>
  <v-main class="v-main-mt">
    <v-container fluid class="v-container-pm fill-height">
      <v-row class="pa-0 ma-0" style="height: 100%">
        <!-- left part -->
        <v-col cols="3" class="pa-0 ma-0 d-flex flex-column" style="max-width: 330px">
          <!-- control components-->
          <v-row class="pa-0 ma-0">
            <v-combobox id="acg" solo clearable hide-details dense :items="directory"
                        label="select a session to use post module"
                        elevation="0"
                        class="cs-nbr"
                        :class="`elevation-0`"
                        style="margin-bottom: 1px; border: 1px solid"
            >
            </v-combobox>
          </v-row>
          <!-- module folder -->
          <v-card class="pa-0 ma-0" height="100%" style="border-radius: 0">
            <v-text-field v-model="search"
                          label="search module"
                          dense
                          solo-inverted
                          hide-details
                          clearable
                          class="cs-nbr"
            ></v-text-field>
            <v-treeview
                :items="directory"
                :search="search"
                open-on-click
            >
              <template v-slot:prepend="{ item }">
                <v-icon
                    v-if="item.children"
                    v-text="`mdi-folder`"
                ></v-icon>
                <v-icon
                    v-else
                    v-text="`mdi-file`"
                ></v-icon>
              </template>
            </v-treeview>

          </v-card>
          <v-text-field
              label="module path"
              readonly
              dense
              solo-inverted
              hide-details
              class="cs-nbr"
          ></v-text-field>
        </v-col>

        <!-- right part -->
        <v-col cols="9" class="pa-0 pl-1 ma-0">
          <v-combobox solo clearable :items="directory" label="Session:"
                      prepend-inner-icon="mdi mdi-bullseye-arrow"

                      class="pa-0 ma-0"
          ></v-combobox>
          <v-btn>Test</v-btn>
        </v-col>
      </v-row>
    </v-container>
  </v-main>
</template>

<script>
import {newLogger} from "../../tool/logger.js"
import fetch from "../../config/fetch.js"

let logger = newLogger("console")

const TYPE_EXPLOIT = 0
const TYPE_AUXILIARY = 1
const TYPE_POST = 2
const TYPE_PAYLOAD = 3
const TYPE_ENCODER = 4
const TYPE_NOP = 5
const TYPE_EVASION = 6

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
      count: types.length,
      directory: directory, // about module directory
      search: null,
    }
  },

  mounted() {
    this.getList();
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
      // reset counter for module id
      this.count = types.length
      // get modules
      for (let i = 0; i < types.length; i++) {
        fetch("GET", `/module/${types[i].path}`).
        then((resp) => {
          let modules = resp.data["modules"]
          this.addModuleToDirectory(modules, types[i].type)
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
        // recover
        current = this.directory[type]
      }
    },

  }
}
</script>

<style type="scss" scoped>

</style>