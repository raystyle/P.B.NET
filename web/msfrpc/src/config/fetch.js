import Vue from "vue"

// create a custom HTTP client
const client = Vue.axios.create({
    baseURL: window.location.origin+"/api",
    headers:{
        "Accept":       "application/json",
        "Content-Type": "application/json",
    },
    timeout: 15000,
})

export default function (method = "GET", path = "", data = {}) {
    return new Promise(function (resolve, reject) {
        switch (method) {
            case "GET":
                try {
                    resolve(client.get(path))
                } catch (err) {
                    reject(err)
                }
                break
            case "POST":
                try {
                    resolve(client.post(path, data))
                } catch (err) {
                    reject(err)
                }
                break
            default:

        }
    })
}