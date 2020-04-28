const login = () => import("../page/login/login")
const session = () => import("../page/session/session")
const job = () => import("../page/job/job")
const console = () => import("../page/console/console")

// test pages for learning Vue.js
const map = () => import("../page/map/map")
const node = () => import("../page/node/node")

export default [
  {
    path: "/",
    redirect: "/session"
  },
  {
    name: "login",
    path: "/login",
    component: login
  },
  {
    name: "session",
    path: "/session",
    component: session,
    meta: {
      showMenu: true,
      keepAlive: true
    }
  },
  {
    name: "job",
    path: "/job",
    component: job,
    meta: {
      showMenu: true,
      keepAlive: true
    }
  },
  {
    name: "console",
    path: "/console",
    component: console,
    meta: {
      showMenu: true,
      keepAlive: true
    }
  },
  {
    name: "map",
    path: "/map",
    component: map,
    meta: {
      showMenu: true,
      keepAlive: true
    }
  },
  {
    name: "node",
    path: "/node",
    component: node,
    meta: {
      showMenu: true,
      keepAlive: true
    }
  },
  {
    path: "*",
    redirect: "/session"
  }
]