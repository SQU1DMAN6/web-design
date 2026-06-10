const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("inker", {
    minimize: function () { ipcRenderer.send("window:minimize"); },
    maximize: function () { ipcRenderer.send("window:maximize"); },
    close: function () { ipcRenderer.send("window:close"); },

    login: function (email, password) { return ipcRenderer.invoke("inker:login", email, password); },
    logout: function () { return ipcRenderer.invoke("inker:logout"); },
    getCurrentUser: function () { return ipcRenderer.invoke("inker:get-current-user"); },

    searchDrops: function (query) { return ipcRenderer.invoke("inker:search-drops", query); },
    listDrops: function () { return ipcRenderer.invoke("inker:list-drops"); },

    mountDrop: function (user, drop, mountPoint) { return ipcRenderer.invoke("inker:mount-drop", user, drop, mountPoint); },
    unmountDrop: function (user, drop) { return ipcRenderer.invoke("inker:unmount-drop", user, drop); },
    getActiveMounts: function () { return ipcRenderer.invoke("inker:get-mounts"); },
    getSavedMounts: function () { return ipcRenderer.invoke("inker:get-saved-mounts"); },
    setAutoMount: function (user, drop, enabled) { return ipcRenderer.invoke("inker:set-auto-mount", user, drop, enabled); },

    getIndex: function (user, drop) { return ipcRenderer.invoke("inker:get-index", user, drop); },
    downloadFile: function (user, drop, relPath) { return ipcRenderer.invoke("inker:download-file", user, drop, relPath); },
    uploadFile: function (user, drop, relPath) { return ipcRenderer.invoke("inker:upload-file", user, drop, relPath); },
    evictFiles: function (user, drop) { return ipcRenderer.invoke("inker:evict-files", user, drop); },

    openPath: function (localPath) { return ipcRenderer.invoke("inker:open-path", localPath); },

    setAutoStart: function (enabled) { return ipcRenderer.invoke("inker:set-autostart", enabled); },
    getAutoStart: function () { return ipcRenderer.invoke("inker:get-autostart"); },

    onLog: function (callback) { ipcRenderer.on("inker:log", function (event, message) { callback(message); }); },
    verifyDrop: function (user, drop) { return ipcRenderer.invoke("inker:verify-drop", user, drop); },

    onReady: function (callback) { ipcRenderer.on("inker:ready", function () { callback(); }); }
});