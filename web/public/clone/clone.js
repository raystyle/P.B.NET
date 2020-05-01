export function deepClone(obj = {}) {
    if (obj === null) {
        return null
    } 
    if (obj instanceof RegExp) {
        return new RegExp(obj)
    }
    if (obj instanceof Date) {
        return new Date(obj)
    }
    if (typeof obj !== "object") {
        return obj
    }
    let t = new obj.constructor();
    for (let key in obj) {
        t[key] = deepClone(obj[key])
    }
    return t
}