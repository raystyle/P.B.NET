module.exports = {
    devServer: {
        port: "8084",
        proxy: {
            "/api/": {
                "target": "https://localhost:17417/",
                "secure": false
            }
        },
        overlay: {
            warnings: true,
            errors: true
        },
    },
    lintOnSave: process.env.NODE_ENV !== "production",
    transpileDependencies: ["vuetify"]
}