import {logLevel} from "@/config/env"

export {
    currentLevel,
    debug,
    info,
    warning,
    error,
    exploit
}

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
    if (v == null) {
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
let currentLevel = parseLevel(logLevel);

function debug(src= "", ...log) {
    printLog(LEVEL_DEBUG, src, log);
}

function info(src= "", log = "") {
    printLog(LEVEL_INFO, src, log);
}

function warning(src= "", log = "") {
    printLog(LEVEL_WARN, src, log);
}

function error(src= "", log = "") {
    printLog(LEVEL_ERROR, src, log);
}

function exploit(src= "", log = "") {
    printLog(LEVEL_EXPLOIT, src, log);
}

function printLog(level = LEVEL_DEBUG, src = "unknown", log) {
    if (level < currentLevel) {
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
    let lv =  levelToString(level);
    // convert log array to string "acg", "foo" => "acg foo"
    let logStr = "";
    for (let i = 0; i < log.length; i++) {
        logStr += " ";
        logStr += log[i].toString();
    }
    console.log(`[${time}] [${lv}] <${src}>${logStr}`);
}