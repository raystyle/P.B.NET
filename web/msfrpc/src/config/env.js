let baseURL = "";
let logLevel = "";

if (process.env.NODE_ENV === "development") {
    baseURL = "https://localhost:17417/api";
    logLevel = "debug"
}else if(process.env.NODE_ENV === "production"){
    baseURL = window.location.origin+"/api";
    logLevel = "info"
}

export {
    baseURL,
    logLevel
}