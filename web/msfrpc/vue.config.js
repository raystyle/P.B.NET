module.exports = {
    devServer: {
        host:"localhost",
        port:"8084",
        proxy: {
            "/api/": {
                target: "https://localhost:17417/",
                secure: false
            }
        }
    }
}
