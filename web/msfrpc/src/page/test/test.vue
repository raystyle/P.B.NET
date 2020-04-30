<template>
  <v-main class="v-main-mt">
    <div class="d-flex flex-row" style="flex-wrap: wrap; max-width: 500px">
      <v-card id="left_part" tile flat height="100%" min-width="310px">
        a
      </v-card>
      <v-divider class="pl-1" vertical style="visibility: hidden"></v-divider>
      <v-card id="right_part" tile flat height="100%" width="500px">
        <v-tabs height="40px" show-arrows style="border: 1px grey solid">
          <v-tabs-slider></v-tabs-slider>

          <v-tab class="v-tab">current</v-tab>
          <v-tab class="v-tab"
                 v-for="i in 30"
                 :key="i"
          >
            Item {{ i }}
          </v-tab>
          <!--
          <v-tabs-items>acg</v-tabs-items>
          <v-tabs-items>acg-2</v-tabs-items>
          <v-tabs-items>acg-3</v-tabs-items>
          -->

        </v-tabs>

        <v-tabs height="40px" show-arrows style="border: 1px grey solid">
          <v-tabs-slider></v-tabs-slider>

          <v-tab class="v-tab">current</v-tab>
          <v-tab class="v-tab"
                 v-for="i in 3"
                 :key="i"
          >
            Item {{ i }}
          </v-tab>
          <!--
          <v-tabs-items>acg</v-tabs-items>
          <v-tabs-items>acg-2</v-tabs-items>
          <v-tabs-items>acg-3</v-tabs-items>
          -->

        </v-tabs>

      </v-card>
    </div>
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

.v-menu__content {
  margin-left: -12px
}

.v-btn {
  height: 90px;
  width: 100px;
  font-size: 16px;
}
</style>