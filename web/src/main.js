import Vue       from 'vue'
import VueRouter from 'vue-router'
import ElementUI from 'element-ui'
import routes    from './router/router'
import App       from './App'

import 'element-ui/lib/theme-chalk/index.css'

Vue.use(VueRouter);
Vue.use(ElementUI);

Vue.config.productionTip = false;

const router = new VueRouter({
  mode: 'hash',
  routes: routes,
});


new Vue({
  router,
  render: h => h(App)
}).$mount('#app');