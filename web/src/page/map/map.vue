<template>
  <el-container>
    <el-header>
      <p>test-000</p>
      <p>test-111</p>

    </el-header>
    <el-main>
      Main
      <p>test-bottom</p>
      <p>test {{hello}}</p>
    </el-main>
  </el-container>
</template>

<script>
    export default {
      data(){
          return{
              hello: 'now',
              timer:null,
          }
      },

        mounted() {
          this.timer = setInterval( ()=> {
              this.hello = 'asd'
          },1000);
       },

        beforeDestroy() {
          clearInterval(this.timer)
        }

    }

</script>

<style scoped>
  .el-container{
    height: 100%;
  }

  .el-header{
    background-color: #B3C0D1;
    color: #333;
    text-align: left;
    height: 300px;
    line-height: 10px;
  }

  .el-main {
    padding: 0px;
    margin: 0px;
    background-color: #E9EEF3;
    color: #333;
    text-align: center;

  }
</style>