import Vue from "vue"
import VueRouter from "vue-router"
import ElementUI from "element-ui"
import Axios from "axios"
import VueAxios from "vue-axios"
import Routes from "./router/router"
import App from "./App"

import "element-ui/lib/theme-chalk/index.css"

Vue.config.productionTip = false;

Vue.use(VueRouter);
Vue.use(ElementUI);
Vue.use(VueAxios, Axios);

import "./page/common/header"

const router = new VueRouter({
  mode: "hash",
  routes: Routes
});

new Vue({
  el: "#app",
  router,
  render: h => h(App)
})