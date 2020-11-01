import fetch from "../config/fetch"

export function getSessionList(noCache = false) {
    let data = {
        no_cache: noCache,
    }
    return fetch("POST", "/session/list", data)
}
