import VueRouter from "vue-router"

// fix error about "Avoided redundant navigation to current location:"
const originalPush = VueRouter.prototype.push
VueRouter.prototype.push = function push(location) {
    return originalPush.call(this, location).catch(err => err)
}

const login = () => import("../page/login/login")
const session = () => import("../page/session/session")
const job = () => import("../page/job/job")
const console = () => import("../page/console/console")
const module = () => import("../page/module/module")
const database = () => import("../page/database/database")
// test pages for learning Vue.js
const map = () => import("../page/map/map")
const node = () => import("../page/node/node")
const test = () => import("../page/test/test")

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
        name: "module",
        path: "/module",
        component: module,
        meta: {
            showMenu: true,
            keepAlive: true
        }
    },
    {
        name: "database",
        path: "/database",
        component: database,
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
        name: "test",
        path: "/test",
        component: test,
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