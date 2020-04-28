import Vue from "vue"
import VueRouter from "vue-router"
import ElementUI from "element-ui"
import Axios from "axios"
import VueAxios from "vue-axios"
import Routes from "./router/router"
import App from "./App"

import "element-ui/lib/theme-chalk/index.css"
import "./page/common/header"

Vue.config.productionTip = false;

Vue.use(VueRouter);
Vue.use(ElementUI);
Vue.use(VueAxios, Axios);

// fix "Avoided redundant navigation to current location:"
const originalPush = VueRouter.prototype.push
VueRouter.prototype.push = function push(location) {
  return originalPush.call(this, location).catch(err => err)
};

const router = new VueRouter({
  mode: "hash",
  routes: Routes
});

new Vue({
  el: "#app",
  router,
  render: h => h(App)
})