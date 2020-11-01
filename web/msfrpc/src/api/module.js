import fetch from "../config/fetch"

export function getModules(type = "") {
    return fetch("GET", `/module/${type}`)
}

export function getModuleInfo(type = "", name = "") {
    let data = {
        type: type,
        name: name
    }
   return fetch("POST", "/module/info", data)
}