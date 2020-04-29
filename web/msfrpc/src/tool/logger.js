import {Logger} from "public/logger/logger"
import {logLevel} from "../config/env"

export function newLogger(src = "unknown") {
    return new Logger(logLevel, src)
}

export default new Logger(logLevel, "global")