<template>
  <v-container>
    <v-btn class="v-btn">Map</v-btn>


  </v-container>
</template>

<script>

export default {
  name: "test",

  props: {
    source: String,
  },

  data: () => ({
    drawer: null,
  }),
}
</script>

<style lang="scss" scoped>
.v-btn {
  max-height: 99px;
min-height: 99px;


  height: 90px;
  width: 100px;
  font-size: 16px;
}
</style>