const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("inker", {
    minimize: () => ipcRenderer.send("window:minimize"),
    maximize: () => ipcRenderer.send("window:maximize"),
    close: () => ipcRenderer.send("window:close"),

    login: (email, password) => ipcRenderer.invoke("inker:login", email, password),
    logout: () => ipcRenderer.invoke("inker:logout"),
    getCurrentUser: () => ipcRenderer.invoke("inker:get-current-user"),

    searchRepositories: (query) => ipcRenderer.invoke("inker:search-repos", query),
    listRepositories: () => ipcRenderer.invoke("inker:list-repos"),

    mountRepository: (user, repo, mountPoint) => ipcRenderer.invoke("inker:mount-repo", user, repo, mountPoint),
    unmountRepository: (user, repo) => ipcRenderer.invoke("inker:unmount-repo", user, repo),
    getActiveMounts: () => ipcRenderer.invoke("inker:get-mounts"),
    getSavedMounts: () => ipcRenderer.invoke("inker:get-saved-mounts"),
    setAutoMount: (user, repo, enabled) => ipcRenderer.invoke("inker:set-auto-mount", user, repo, enabled),

    openPath: (localPath) => ipcRenderer.invoke("inker:open-path", localPath),

    setAutoStart: (enabled) => ipcRenderer.invoke("inker:set-autostart", enabled),
    getAutoStart: () => ipcRenderer.invoke("inker:get-autostart"),

    onLog: (callback) => ipcRenderer.on("inker:log", (event, message) => callback(message)),
    onReady: (callback) => ipcRenderer.on("inker:ready", () => callback())
});