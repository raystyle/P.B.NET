<template>
  <v-main class="v-main-mt">
    <v-container fluid class="v-container-pm fill-height">

      <v-menu id="spe" offset-y allow-overflow  style="margin-left: -12px">
        <template v-slot:activator="{ on, attrs }" >
          <v-btn
              color="primary"
              dark
              v-bind="attrs"
              v-on="on"


          >
            Dropdown
          </v-btn>
        </template>


        <v-list >
          <v-list-item
              v-for="(item, index) in items"
              :key="index"


          >
            <v-list-item-title>{{ item.title }}</v-list-item-title>
          </v-list-item>
        </v-list>
      </v-menu>

      <v-btn elevation="0" >Menu</v-btn>



      <v-menu offset-y>
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
        rounded: "0",
      },

      items: [
        { title: 'Click Me' },
        { title: 'Click Me' },
        { title: 'Click Me' },
      ],
    }
  }
}
</script>

<style lang="scss" scoped>

#spe {
}

.v-menu__content {
  margin-left: -12px
}

.v-btn {
  height: 90px;
  width: 100px;
  font-size: 16px;
}
</style>