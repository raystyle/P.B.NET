import {formatDate} from "public/time/time"

// about log level
export const LEVEL_DEBUG   = 1
export const LEVEL_INFO    = 2
export const LEVEL_WARN    = 3
export const LEVEL_ERROR   = 4
export const LEVEL_EXPLOIT = 5

function parseLevel(level = "") {
    switch (level.toLowerCase()) {
        case "debug":
            return LEVEL_DEBUG
        case  "info":
            return LEVEL_INFO
        case "warning":
            return LEVEL_WARN
        case "error":
            return LEVEL_ERROR
        case "exploit":
            return LEVEL_EXPLOIT
        default:
            return LEVEL_DEBUG
    }
}

function levelToString(level = LEVEL_DEBUG) {
    switch (level) {
        case LEVEL_DEBUG:
            return "debug"
        case  LEVEL_INFO:
            return "info"
        case LEVEL_WARN:
            return "warning"
        case LEVEL_ERROR:
            return "error"
        case LEVEL_EXPLOIT:
            return "exploit"
        default:
            return "unknown"
    }
}

// Logger is used to record log about components.
// You can set the minimum level to filter log.
export class Logger {
    constructor(level = "info", src = "unknown") {
        this._level = parseLevel(level)
        this._src = src
    }

    setLevel(lv) {
        this._level = lv
    }

    getLevel() {
        return this._level
    }

    debug(...log) {
        this._print(LEVEL_DEBUG, log)
    }

    info(...log) {
        this._print(LEVEL_INFO, log)
    }

    warning(...log) {
        this._print(LEVEL_WARN, log)
    }

    error(...log) {
        this._print(LEVEL_ERROR, log)
    }

    exploit(...log) {
        this._print(LEVEL_EXPLOIT, log)
    }

    _print(lv, log) {
        if (lv < this._level) {
            return
        }
        // get time string
        let time = formatDate(new Date())
        // get level string
        let lvStr =  levelToString(lv)
        // convert log array to string "acg", "foo" => "acg foo"
        let logStr = ""
        for (let i = 0; i < log.length; i++) {
            logStr += " "
            logStr += log[i].toString()
        }
        console.log(`[${time}] [${lvStr}] <${this._src}>${logStr}`)
    }
}