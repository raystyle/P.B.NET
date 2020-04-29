<template>
  <v-main class="v-main-mt">
    <v-container fluid class="v-container-pm fill-height">
      <v-row class="pa-0 ma-0" style="height: 100%">
        <!-- left part -->
        <v-col cols="3" class="pa-0 ma-0 d-flex flex-column" style="max-width: 330px">
          <!-- control components-->
          <v-row class="pa-0 ma-0">
            <v-combobox solo clearable hide-details dense :items="moduleDir"
                        allow-overflow absolute
                        label="select a session to use post module"

                        style="padding-bottom: 1px; border-radius: 0"
            >
            </v-combobox>

          </v-row>
          <!-- module folder -->
          <v-card height="100%" class="pa-0 ma-0" style="border-radius: 0">
            <v-sheet class="primary lighten-2">
              <v-text-field
                  v-model="search"
                  label="Search Company Directory"
                  dark
                  flat
                  solo-inverted
                  hide-details
                  clearable
                  clear-icon="mdi-close-circle-outline"
              ></v-text-field>
              <v-checkbox
                  v-model="caseSensitive"
                  dark
                  hide-details
                  label="Case sensitive search"
              ></v-checkbox>
            </v-sheet>
            <v-card-text>
              <v-treeview
                  :items="moduleDir"
                  :search="search"
                  :filter="filter"
                  :open.sync="open"
              >
                <template v-slot:prepend="{ item }">
                  <v-icon
                      v-if="item.children"
                      v-text="`mdi-${item.id === 1 ? 'home-variant' : 'folder-network'}`"
                  ></v-icon>
                </template>
              </v-treeview>
            </v-card-text>
          </v-card>
        </v-col>

        <!-- right part -->
        <v-col cols="9" class="pa-0 pl-1 ma-0">
          <v-combobox solo clearable :items="moduleDir" label="Session:"
                      prepend-inner-icon="mdi mdi-bullseye-arrow"

                      class="pa-0 ma-0"
          ></v-combobox>
          <v-btn>Test</v-btn>
        </v-col>
      </v-row>
    </v-container>
  </v-main>
</template>

d-flex flex-column

<script>
export default {
  name: "module",

  data: () => ({
    moduleDir: [
      {
        id: 0,
        name: "exploits"
      },

      {
        id: 8,
        name: "auxiliary"
      },

      {
        id: 9,
        name: "post"
      },


      {
        id: 1,
        name: 'Vuetify Human Resources',
        children: [
          {
            id: 2,
            name: 'Core team',
            children: [
              {
                id: 201,
                name: 'John',
              },
              {
                id: 202,
                name: 'Kate',
              },
              {
                id: 203,
                name: 'Neko',
              },
              {
                id: 204,
                name: 'Jacek',
              },
              {
                id: 205,
                name: 'Andrew',
              },
            ],
          },
          {
            id: 3,
            name: 'Administrators',
            children: [
              {
                id: 301,
                name: 'Ranee',
              },
              {
                id: 302,
                name: 'Rachel',
              },
            ],
          },
          {
            id: 4,
            name: 'Contributors',
            children: [
              {
                id: 401,
                name: 'Pho',
              },
              {
                id: 402,
                name: 'Brandon',
              },
              {
                id: 403,
                name: 'Sean',
              },
            ],
          },
        ],
      },
    ],
    open: [0, 2],
    search: null,
    caseSensitive: false,


  }),
  computed: {
    filter () {
      return this.caseSensitive
          ? (item, search, textKey) => item[textKey].indexOf(search) > -1
          : undefined
    },
  },
}
</script>

<style type="scss" scoped>

</style>