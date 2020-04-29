import Vue from "vue"
import VueRouter from "vue-router"
import Axios from "axios"
import VueAxios from "vue-axios"
import Routes from "./router/router"
import vuetify from "./plugin/vuetify"
import App from "./App"

import "./page/common/menu"

Vue.config.productionTip = false

Vue.use(VueRouter)
Vue.use(VueAxios, Axios)

const router = new VueRouter({
    mode: "hash",
    routes: Routes
})

new Vue({
    el: "#app",
    vuetify,
    router,
    render: h => h(App)
})