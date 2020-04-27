let logLevel = "";

if (process.env.NODE_ENV === "development") {
    logLevel = "debug"
} else if(process.env.NODE_ENV === "production"){
    logLevel = "info"
}

export {
    logLevel
}