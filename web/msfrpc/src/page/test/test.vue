<template>
  <v-main class="v-main-mt">
    <v-container fluid class="v-container-pm fill-height">
      <v-combobox  solo clearable hide-details dense

                   class="pa-0 ma-0"
                   :items="items"
                   label="select session to use post module" style="border-radius: 0;"

                   :menu-props="{rounded:10}"

      >
      </v-combobox>



      <v-menu offset-y allow-overflow offset-overflow>
        <template v-slot:activator="{ on, attrs }">
          <v-btn
              color="primary"
              dark
              v-bind="attrs"
              v-on="on"
          >
            Dropdown
          </v-btn>
        </template>
        <v-list>
          <v-list-item
              v-for="(item, index) in items"
              :key="index"
          >
            <v-list-item-title>{{ item.title }}</v-list-item-title>
          </v-list-item>
        </v-list>
      </v-menu>

      <v-btn>Menu</v-btn>

      <v-menu offset-y rounded="0" style="left: 100px">
        <template v-slot:activator="{ on, attrs }">
          <v-btn
              color="primary"
              dark
              v-bind="attrs"
              v-on="on"
          >
            Dropdown
          </v-btn>
        </template>
        <v-list>
          <v-list-item
              v-for="(item, index) in items"
              :key="index"
          >
            <v-list-item-title>{{ item.title }}</v-list-item-title>
          </v-list-item>
        </v-list>
      </v-menu>




    </v-container>
  </v-main>
</template>

<script>

export default {
  name: "test",

  data () {
    return {
      pp: {
        rounded: "10 ",
      },

      items: [
        { title: 'Click Me' },
        { title: 'Click Me' },
        { title: 'Click Me' },
        { title: 'Click Me 2' },
      ],
    }
  }
}
</script>

<style lang="scss" scoped>
.v-btn {
  height: 90px;
  width: 100px;
  font-size: 16px;
}
</style>