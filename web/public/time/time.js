export function formatDate(date = Date(), format){
    let padNum = function(num){
        num += ""
        return num.replace(/^(\d)$/,"0$1")
    }
    let cfg = {
        yyyy : date.getFullYear(),
        yy : date.getFullYear().toString().substring(2),
        M  : date.getMonth() + 1,
        MM : padNum(date.getMonth() + 1),
        d  : date.getDate(),
        dd : padNum(date.getDate()),
        hh : padNum(date.getHours()),
        mm : padNum(date.getMinutes()),
        ss : padNum(date.getSeconds())
    }
    format || (format = "yyyy-MM-dd hh:mm:ss")
    return format.replace(/([a-z])(\1)*/ig, function (m) {return cfg[m]})
}