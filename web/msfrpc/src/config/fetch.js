import Vue from "vue";
import {baseURL} from "./env";

// create a custom HTTP client
const client = Vue.axios.create({
    baseURL: baseURL,
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
                client.get(path)

                resolve()
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