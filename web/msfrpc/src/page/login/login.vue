<template>
  <div>
    <b>login</b>
    <p>test {{ hello }}</p>
  </div>
</template>

<script>
export default {
  name:"login",

  data() {
    console.log("loaded 1")
    return {
      hello: "now",
      value: 1
    };
  },

  mounted() {
    console.log("mounted 1")

    this.timer = setInterval(() => {
      this.value += 1;
      this.hello = this.value;
    }, 1000);
  },

  beforeDestroy() {
    clearInterval(this.timer);
  }
}
</script>

<style scoped>

</style>
