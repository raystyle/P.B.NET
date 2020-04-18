<template>
  <el-container>
    <c-header></c-header>
    <el-header>
      <p>test-000</p>
      <p>test-111</p>
    </el-header>
    <el-main>
      <p>Main</p>
      <p>test-bottom</p>
      <p>test {{ hello }}</p>
    </el-main>
  </el-container>
</template>

<script>
export default {
  data() {
    return {
      hello: "now",
      value: 1,
      timer: null
    };
  },

  mounted() {
    this.timer = setInterval(() => {
      this.value += 1;
      this.hello = this.value;
    }, 1000);
  },
  beforeDestroy() {
    clearInterval(this.timer);
  }
};
</script>

<style scoped>
.el-container {
  padding: 0;
  margin: 0;
  height: 100%;
}

.el-header {
  padding: 0;
  margin: 0;
  background-color: #b3c0d1;
  color: #333;
  text-align: left;
  height: 300px;
  line-height: 10px;
}

.el-main {
  padding: 0;
  margin: 0;
  background-color: #e9eef3;
  color: #333;
  text-align: center;
}
</style>
