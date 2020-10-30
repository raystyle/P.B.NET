import Vue from "vue"
import VueRouter from "vue-router"
import Axios from "axios"
import VueAxios from "vue-axios"
import Routes from "./router/router"
import Vuetify from "./plugin/vuetify"
import App from "./app"

import "./page/common/menu"

Vue.config.productionTip = false

Vue.use(VueRouter)
Vue.use(VueAxios, Axios)

const Router = new VueRouter({
    mode: "hash",
    routes: Routes,
})

new Vue({
    el: "#app",
    vuetify: Vuetify,
    router: Router,
    render: h => h(App),
})