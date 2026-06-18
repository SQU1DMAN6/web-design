const { app, BrowserWindow, ipcMain, shell, Tray, Menu, nativeImage } = require("electron");
const path = require("path");
const os = require("os");
const fs = require("fs");

const apiClient = require("./lib/api-client-v2");
const VirtualFS = require("./lib/virtual-fs");
const Hydrator = require("./lib/hydrator");
const WinFspBridge = require("./lib/winfsp-bridge");
const settings = require("./lib/settings");

let win;
let tray;
let hydrator;
let virtualFs;
let winfspBridge;
let appReady = false;
let isQuitting = false;

const gotTheLock = app.requestSingleInstanceLock();
if (!gotTheLock) {
    app.quit();
}

function createWindow() {
    win = new BrowserWindow({
        width: 900,
        height: 650,
        minWidth: 750,
        minHeight: 500,
        frame: false,
        transparent: true,
        backgroundColor: "#00000000",
        icon: path.join(__dirname, "icon.ico"),
        show: false,
        webPreferences: {
            preload: path.join(__dirname, "preload.js"),
            contextIsolation: true,
            nodeIntegration: false
        }
    });
    win.loadFile("renderer/index.html");

    win.once("ready-to-show", function () {
        win.show();
    });

    win.on("close", function (e) {
        if (!isQuitting) {
            e.preventDefault();
            win.hide();
            logToRenderer("Window hidden to tray (filesystem mounts continue)");
        }
    });
}

function createTray() {
    var iconPath = path.join(__dirname, "icon.ico");
    var trayIcon = nativeImage.createFromPath(iconPath);
    tray = new Tray(trayIcon);
    tray.setToolTip("FtR Inker — InkDrop Drive");

    // Initial placeholder menu — no mounts yet
    var placeholderMenu = Menu.buildFromTemplate([
        { label: "Open FtR Inker", click: function () { if (win) { if (win.isMinimized()) win.restore(); win.show(); win.focus(); } } },
        { label: "Open InkDrop Drive", enabled: false },
        { type: "separator" },
        { label: "Exit FtR Inker", click: function () { isQuitting = true; app.quit(); } }
    ]);
    tray.setContextMenu(placeholderMenu);

    tray.on("double-click", function () {
        if (win) {
            if (win.isMinimized()) win.restore();
            win.show();
            win.focus();
        }
    });
}

/** Call this AFTER winfspBridge is initialized to wire up dynamic tray menu */
function setupTrayMenuRebuild() {
    if (!winfspBridge || !tray) return;

    function rebuildTrayMenu() {
        var template = [
            {
                label: "Open FtR Inker",
                click: function () {
                    if (win) {
                        if (win.isMinimized()) win.restore();
                        win.show();
                        win.focus();
                    }
                }
            }
        ];

        var mountPaths = [];
        try {
            var mounts = winfspBridge.getActiveMounts();
            for (var mi = 0; mi < mounts.length; mi++) {
                var mp = (mounts[mi].mountPath || "").trim();
                if (mp && mountPaths.indexOf(mp) === -1) {
                    mountPaths.push(mp);
                }
            }
        } catch (e) {
            // Bridge not fully ready
        }

        if (mountPaths.length > 0) {
            for (var pi = 0; pi < mountPaths.length; pi++) {
                (function (p) {
                    template.push({
                        label: "Open " + p,
                        click: function () { shell.openPath(p); }
                    });
                })(mountPaths[pi]);
            }
        } else {
            template.push({ label: "Open InkDrop Drive", enabled: false });
        }

        template.push({ type: "separator" });
        template.push({
            label: "Exit FtR Inker",
            click: function () { isQuitting = true; app.quit(); }
        });

        try { tray.setContextMenu(Menu.buildFromTemplate(template)); } catch (e) {}
    }

    winfspBridge.on("mounted", rebuildTrayMenu);
    winfspBridge.on("unmounted", rebuildTrayMenu);

    // Initial rebuild after events are wired
    rebuildTrayMenu();
}

function logToRenderer(message) {
    if (!win || win.isDestroyed()) return;
    var ts = new Date().toISOString().replace("T", " ").slice(0, 19);
    console.log("[INKER] " + ts + " " + message);
    if (win.webContents) {
        try { win.webContents.send("inker:log", message); } catch (e) { /* ignore */ }
    }
}

async function initializeApp() {
    logToRenderer("Initializing Inker 3.2 — InkDrop Drive...");
    try {
        hydrator = new Hydrator(apiClient);
        virtualFs = new VirtualFS(hydrator);
        winfspBridge = new WinFspBridge(virtualFs);

        winfspBridge.on("mounted", function (data) {
            logToRenderer("Mounted " + data.dropKey + " at " + data.mountPath);
        });

        winfspBridge.on("unmounted", function (data) {
            logToRenderer("Unmounted " + data.dropKey);
        });

        // Wire up dynamic tray menu now that winfspBridge exists
        setupTrayMenuRebuild();

        var auth = await settings.getAuth();
        if (auth) {
            apiClient.setSession(auth.email, auth.username, auth.session_id);
            logToRenderer("Saved session found for " + auth.username);

            var sessionResult = await apiClient.sessionConfirm();
            if (sessionResult.valid) {
                logToRenderer("Session confirmed: " + sessionResult.username);
                if (sessionResult.username && sessionResult.username !== auth.username) {
                    await settings.saveAuth(auth.email, sessionResult.username, apiClient.sessionId);
                }

                var savedMounts = await settings.getMounts();
                logToRenderer("Found " + savedMounts.length + " saved mount(s)");
                for (var mi = 0; mi < savedMounts.length; mi++) {
                    var mount = savedMounts[mi];
                    try {
                        var dropKey = mount.user + "/" + mount.drop;
                        logToRenderer("Mounting " + dropKey + " on InkDrop Drive");
                        await winfspBridge.mount(dropKey, mount.user, mount.drop, "Q:\\");
                    } catch (err) {
                        logToRenderer("Failed to mount " + mount.user + "/" + mount.drop + ": " + err.message);
                    }
                }

                appReady = true;
                if (win && !win.isDestroyed()) win.webContents.send("inker:ready", {
                    valid: true,
                    email: sessionResult.email || auth.email,
                    username: sessionResult.username || auth.username
                });
            } else {
                logToRenderer("Session expired on server");
                await settings.clearAuth();
                appReady = true;
                if (win && !win.isDestroyed()) win.webContents.send("inker:ready", { valid: false });
            }
        } else {
            logToRenderer("No saved session found");
            appReady = true;
            if (win && !win.isDestroyed()) win.webContents.send("inker:ready", { valid: false });
        }

        logToRenderer("Inker 3.2 initialized successfully");
    } catch (err) {
        logToRenderer("Initialization error: " + err.message);
        appReady = true;
        if (win && !win.isDestroyed()) win.webContents.send("inker:ready", { valid: false });
    }
}

// ====== IPC Handlers ======

ipcMain.handle("inker:login", async function (event, email, password) {
    try {
        logToRenderer("Attempting login for " + email);
        var result = await apiClient.login(email, password);
        await settings.saveAuth(email, result.username, apiClient.sessionId);
        logToRenderer("Login confirmed: " + result.username);
        return { success: true, username: result.username, email: email };
    } catch (err) {
        logToRenderer("Login failed: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:logout", async function () {
    try {
        logToRenderer("Logging out...");
        if (winfspBridge) await winfspBridge.unmountAll();
        await settings.clearAuth();
        apiClient.sessionId = null;
        apiClient.email = null;
        apiClient.username = null;
        logToRenderer("Logout successful");
        return { success: true };
    } catch (err) {
        logToRenderer("Logout error: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:get-session", async function () {
    var auth = await settings.getAuth();
    if (auth && apiClient.sessionId) {
        var result = await apiClient.sessionConfirm();
        if (result.valid) {
            return { email: result.email || auth.email, username: result.username || auth.username };
        }
        await settings.clearAuth();
        apiClient.sessionId = null;
        return null;
    }
    return null;
});

ipcMain.handle("inker:search-drops", async function (event, query) {
    try {
        logToRenderer("Searching for: " + query);
        var results = await apiClient.searchDrops(query);
        logToRenderer("Found " + results.length + " result(s)");
        return results;
    } catch (err) {
        logToRenderer("Search failed: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:mount-drop", async function (event, user, drop) {
    try {
        if (!winfspBridge) {
            throw new Error("Filesystem engine not ready yet");
        }

        var dropKey = user + "/" + drop;
        logToRenderer("Mounting " + dropKey + " on InkDrop Drive");

        var activeMounts = winfspBridge.getActiveMounts();
        var alreadyMounted = false;
        for (var i = 0; i < activeMounts.length; i++) {
            if (activeMounts[i].dropKey === dropKey) {
                alreadyMounted = true;
                break;
            }
        }

        if (alreadyMounted) {
            logToRenderer(dropKey + " already mounted");
            return { dropKey: dropKey, mountPath: "Q:\\" };
        }

        var mountPath = await winfspBridge.mount(dropKey, user, drop, "Q:\\");
        await settings.addMount(user, drop, mountPath, true);
        logToRenderer("Mounted " + dropKey);
        return { dropKey: dropKey, mountPath: mountPath };
    } catch (err) {
        logToRenderer("Mount failed: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:unmount-drop", async function (event, user, drop) {
    try {
        if (!winfspBridge) {
            throw new Error("Filesystem engine not ready yet");
        }

        var dropKey = user + "/" + drop;
        logToRenderer("Unmounting " + dropKey);

        var activeMounts = winfspBridge.getActiveMounts();
        var found = false;
        for (var i = 0; i < activeMounts.length; i++) {
            if (activeMounts[i].dropKey === dropKey) {
                found = true;
                break;
            }
        }
        if (!found) {
            throw new Error(dropKey + " is not mounted");
        }

        await winfspBridge.unmount(dropKey);
        await settings.removeMount(dropKey);
        logToRenderer("Unmounted " + dropKey);
        return { success: true };
    } catch (err) {
        logToRenderer("Unmount failed: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:get-mounts", async function () {
    try {
        return winfspBridge ? winfspBridge.getActiveMounts() : [];
    } catch (err) {
        logToRenderer("Get mounts error: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:get-saved-mounts", async function () {
    try {
        return await settings.getMounts();
    } catch (err) {
        logToRenderer("Get saved mounts error: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:set-auto-mount", async function (event, user, drop, enabled) {
    try {
        var dropPath = user + "/" + drop;
        await settings.setAutoMount(dropPath, enabled);
        logToRenderer("Auto-mount " + (enabled ? "enabled" : "disabled") + " for " + dropPath);
        return { success: true };
    } catch (err) {
        logToRenderer("Set auto-mount error: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:open-path", async function (event, localPath) {
    try {
        logToRenderer("Opening: " + localPath);
        await shell.openPath(localPath);
        return { path: localPath };
    } catch (err) {
        logToRenderer("Open failed: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:open-external", async function (event, url) {
    try {
        logToRenderer("Opening external: " + url);
        await shell.openExternal(url);
        return { success: true };
    } catch (err) {
        logToRenderer("Open external failed: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:get-cache-info", async function () {
    try {
        if (!hydrator) return { size: 0, entries: 0 };
        return hydrator.getCacheStats();
    } catch (err) {
        return { size: 0, entries: 0 };
    }
});

ipcMain.handle("inker:clear-cache", async function () {
    try {
        if (hydrator) hydrator.clearCaches();
        logToRenderer("Cache cleared");
        return { success: true };
    } catch (err) {
        logToRenderer("Clear cache error: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:hide-window", function () {
    if (win) win.hide();
    return { success: true };
});

ipcMain.handle("inker:show-window", function () {
    if (win) {
        if (win.isMinimized()) win.restore();
        win.show();
        win.focus();
    }
    return { success: true };
});

ipcMain.on("window:minimize", function () {
    if (win) win.minimize();
});

ipcMain.on("window:maximize", function () {
    if (win) {
        if (win.isMaximized()) {
            win.unmaximize();
        } else {
            win.maximize();
        }
    }
});

ipcMain.on("window:close", function () {
    if (win) win.hide();
});

app.whenReady().then(function () {
    logToRenderer("Electron app ready");
    createWindow();
    createTray();
    initializeApp();
});

app.on("before-quit", function () {
    if (win && !win.isDestroyed()) {
        win.removeAllListeners("close");
    }
    logToRenderer("Shutting down...");
    if (winfspBridge) {
        winfspBridge.unmountAll();
    }
    if (tray) tray.destroy();
});

app.on("window-all-closed", function () {
    // Don't quit — we have a tray
});