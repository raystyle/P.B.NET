import Vue from "vue"

// create a custom HTTP client
const client = Vue.axios.create({
    baseURL: window.location.origin+"/api",
    headers:{
        "Accept":       "application/json",
        "Content-Type": "application/json"
    },
    timeout: 15000
});

export default async(method = "GET", path = "", data = {}) => {
    return new Promise(function (resolve, reject) {
        switch (method) {
            case "GET":
                try {
                    resolve(client.get(path))
                }catch (e) {
                    reject(e)
                }
                break;
            case "POST":
                client.post(path, data)
                reject()
                break;
            default:
                console.log()
        }

    })
}