const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("inker", {
    // Window controls
    minimize: function () { ipcRenderer.send("window:minimize"); },
    maximize: function () { ipcRenderer.send("window:maximize"); },
    close: function () { ipcRenderer.send("window:close"); },

    // Auth
    login: function (email, password) { return ipcRenderer.invoke("inker:login", email, password); },
    logout: function () { return ipcRenderer.invoke("inker:logout"); },
    getSession: function () { return ipcRenderer.invoke("inker:get-session"); },

    // Drops
    searchDrops: function (query) { return ipcRenderer.invoke("inker:search-drops", query); },

    // Mount management
    mountDrop: function (user, drop) { return ipcRenderer.invoke("inker:mount-drop", user, drop); },
    unmountDrop: function (user, drop) { return ipcRenderer.invoke("inker:unmount-drop", user, drop); },
    getActiveMounts: function () { return ipcRenderer.invoke("inker:get-mounts"); },
    getSavedMounts: function () { return ipcRenderer.invoke("inker:get-saved-mounts"); },
    setAutoMount: function (user, drop, enabled) { return ipcRenderer.invoke("inker:set-auto-mount", user, drop, enabled); },

    // File system
    openPath: function (localPath) { return ipcRenderer.invoke("inker:open-path", localPath); },
    openExternal: function (url) { return ipcRenderer.invoke("inker:open-external", url); },

    // Cache (minimal — invisible by default)
    getCacheInfo: function () { return ipcRenderer.invoke("inker:get-cache-info"); },
    clearCache: function () { return ipcRenderer.invoke("inker:clear-cache"); },

    // Window visibility
    hideWindow: function () { return ipcRenderer.invoke("inker:hide-window"); },
    showWindow: function () { return ipcRenderer.invoke("inker:show-window"); },

    // Event listeners
    onLog: function (callback) { ipcRenderer.on("inker:log", function (event, message) { callback(message); }); },
    onReady: function (callback) { ipcRenderer.on("inker:ready", function (event, data) { callback(data); }); }
});