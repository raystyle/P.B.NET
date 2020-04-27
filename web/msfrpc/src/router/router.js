const login = () => import("../page/login/login")
const session = () => import("../page/session/session")
const job = () => import("../page/job/job")
const console = () => import("../page/console/console")
const map = () => import("../page/map/map")
const node = () => import("../page/node/node")

export default [
  {
    path: "/",
    redirect: "/session"
  },
  {
    path: "/login",
    component: login
  },
  {
    path: "/session",
    component: session,
    meta: {
      menu: true,
      keepAlive: true
    }
  },
  {
    path: "/job",
    component: job,
    meta: {
      menu: true,
      keepAlive: true
    }
  },
  {
    path: "/console",
    component: console,
    meta: {
      menu: true,
      keepAlive: true
    }
  },
  {
    path: "/map",
    component: map,
    meta: {
      menu: true,
      keepAlive: true
    }
  },
  {
    path: "/node",
    component: node,
    meta: {
      menu: true,
      keepAlive: true
    }
  },
  {
    path: "*",
    redirect: "/session"
  }
]