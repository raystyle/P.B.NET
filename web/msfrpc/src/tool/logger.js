import {logLevel} from "../config/env"

// log level
const LEVEL_DEBUG   = 1;
const LEVEL_INFO    = 2;
const LEVEL_WARN    = 3;
const LEVEL_ERROR   = 4;
const LEVEL_EXPLOIT = 5;

const levelToNumber = {
    debug:   LEVEL_DEBUG,
    info:    LEVEL_INFO,
    warning: LEVEL_WARN,
    error:   LEVEL_ERROR,
    exploit: LEVEL_EXPLOIT
};

function parseLevel(level = "") {
    let v = levelToNumber[level];
    if (!v) {
        v = LEVEL_DEBUG;
    }
    return v;
}

function levelToString(level = LEVEL_DEBUG) {
    switch (level) {
        case LEVEL_DEBUG:
            return "debug";
        case  LEVEL_INFO:
            return "info";
        case LEVEL_WARN:
            return "warning";
        case LEVEL_ERROR:
            return "error";
        case LEVEL_EXPLOIT:
            return "exploit";
        default:
            return "unknown";
    }
}

// user can change it
let level = parseLevel(logLevel);

function debug(src= "", ...log) {
    printLog(LEVEL_DEBUG, src, log);
}

function info(src= "", ...log) {
    printLog(LEVEL_INFO, src, log);
}

function warning(src= "", ...log) {
    printLog(LEVEL_WARN, src, log);
}

function error(src= "", ...log) {
    printLog(LEVEL_ERROR, src, log);
}

function exploit(src= "", ...log) {
    printLog(LEVEL_EXPLOIT, src, log);
}

function printLog(lv = LEVEL_DEBUG, src = "unknown", log) {
    if (lv < level) {
        return
    }
    // get time string
    let date = new Date();
    let year = date.getFullYear();
    let month = date.getMonth();
    let day = date.getDate();
    let hour = date.getHours();
    let minute = date.getMinutes();
    let second = date.getSeconds();
    let time = `${year}-${month}-${day} ${hour}:${minute}:${second}`;
    // get level string
    let lvStr =  levelToString(lv);
    // convert log array to string "acg", "foo" => "acg foo"
    let logStr = "";
    for (let i = 0; i < log.length; i++) {
        logStr += " ";
        logStr += log[i].toString();
    }
    console.log(`[${time}] [${lvStr}] <${src}>${logStr}`);
}

export default {
    LEVEL_DEBUG,
    LEVEL_INFO,
    LEVEL_WARN,
    LEVEL_ERROR,
    LEVEL_EXPLOIT,
    level,
    debug,
    info,
    warning,
    error,
    exploit
}
