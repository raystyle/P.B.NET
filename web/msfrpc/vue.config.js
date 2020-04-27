module.exports = {
    devServer: {
        port:"8084",
        proxy: {
            "/api/": {
                target: "https://localhost:17417/",
                secure: false
            }
        }
    }
}
