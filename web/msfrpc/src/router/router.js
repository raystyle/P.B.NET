const login = () => import("@/page/login/login");
const map = () => import("@/page/map/map");
const node = () => import("@/page/node/node");

export default [
  {
    path: "/",
    redirect: "/map"
  },
  {
    path: "/login",
    component: login
  },
  {
    path: "/map",
    component: map,
    meta: {
      keepAlive: true
    }
  },
  {
    path: "/node",
    component: node,
    meta: {
      keepAlive: true
    }
  },
  {
    path: "*",
    redirect: "/map"
  }
];
