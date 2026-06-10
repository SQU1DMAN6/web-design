const { app, BrowserWindow, ipcMain, shell } = require("electron");
const path = require("path");
const os = require("os");
const fs = require("fs");

const apiClient = require("./lib/api-client");
const MountManager = require("./lib/mount-manager");
const SyncManager = require("./lib/sync-manager");
const settings = require("./lib/settings");

let win;
let mountManager;
let syncManager;
let appReady = false;

const gotTheLock = app.requestSingleInstanceLock();
if (!gotTheLock) {
    app.quit();
}

function createWindow() {
    win = new BrowserWindow({
        width: 1000,
        height: 700,
        minWidth: 800,
        minHeight: 600,
        frame: false,
        transparent: true,
        backgroundColor: "#00000000",
        webPreferences: {
            preload: path.join(__dirname, "preload.js"),
            contextIsolation: true,
            nodeIntegration: false
        }
    });

    win.loadFile("renderer/index.html");
}

function logToRenderer(message) {
    const ts = new Date().toISOString().replace("T", " ").slice(0, 19);
    console.log("[INKER] " + ts + " " + message);
    if (win && win.webContents) {
        win.webContents.send("inker:log", message);
    }
}

async function initializeApp() {
    logToRenderer("Initializing application...");
    try {
        mountManager = new MountManager(apiClient);
        syncManager = new SyncManager(apiClient);

        mountManager.on("mounted", function (data) {
            logToRenderer(" Mounted " + data.dropPath + " at " + data.mountPoint);
            syncManager.startSync(data.dropPath, data.mountPoint);
        });

        mountManager.on("unmounted", function (data) {
            logToRenderer(" Unmounted " + data.dropPath);
            syncManager.stopSync(data.dropPath);
        });

        syncManager.on("file-synced", function (data) {
            logToRenderer("[sync] " + data.type.toUpperCase() + " " + data.dropPath + "/" + data.file);
        });

        syncManager.on("sync-error", function (data) {
            logToRenderer("[sync error] " + data.dropPath + "/" + data.file + ": " + data.error);
        });

        const auth = await settings.getAuth();
        if (auth) {
            apiClient.setSession(auth.email, auth.username, auth.session_id);
            logToRenderer("Loaded session for " + auth.username);

            const mounts = await settings.getMounts();
            logToRenderer("Found " + mounts.length + " saved mount(s)");
            for (const mount of mounts) {
                if (mount.auto_mount) {
                    try {
                        logToRenderer("Auto-mounting " + mount.drop_path + " at " + mount.mount_point);
                        await mountManager.mount(mount.user, mount.drop, mount.mount_point);
                    } catch (err) {
                        logToRenderer("Failed to auto-mount " + mount.drop_path + ": " + err.message);
                    }
                }
            }
        } else {
            logToRenderer("No saved session found");
        }

        appReady = true;
        if (win) win.webContents.send("inker:ready");
        logToRenderer("Application initialized successfully");
    } catch (err) {
        logToRenderer("Initialization error: " + err.message);
    }
}

ipcMain.handle("inker:login", async function (event, email, password) {
    try {
        logToRenderer("Attempting login for " + email);
        const result = await apiClient.login(email, password);
        await settings.saveAuth(email, result.username, apiClient.sessionId);
        logToRenderer("Login successful: " + result.username);
        return { success: true, username: result.username, email: email };
    } catch (err) {
        logToRenderer("Login failed: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:logout", async function () {
    try {
        logToRenderer("Logging out...");
        mountManager.closeAll();
        syncManager.stopAllSync();
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

ipcMain.handle("inker:get-current-user", async function () {
    const auth = await settings.getAuth();
    if (auth) {
        return { email: auth.email, username: auth.username };
    }
    return null;
});

ipcMain.handle("inker:search-drops", async function (event, query) {
    try {
        logToRenderer("Searching Drops for: " + query);
        const results = await apiClient.searchDrops(query);
        logToRenderer("Search returned " + results.length + " result(s)");
        return results;
    } catch (err) {
        logToRenderer("Search failed: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:list-drops", async function () {
    try {
        logToRenderer("Listing all Drops...");
        const results = await apiClient.listDrops();
        logToRenderer("List returned " + results.length + " Drop(s)");
        return results;
    } catch (err) {
        logToRenderer("List Drops failed: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:verify-drop", async function (event, user, drop) {
    try {
        logToRenderer("Verifying Drop " + user + "/" + drop + "...");
        var result = await apiClient.verifyDrop(user, drop);
        logToRenderer("Drop " + user + "/" + drop + " verification: " + (result ? "exists" : "not found"));
        return result;
    } catch (err) {
        logToRenderer("Verify Drop failed: " + err.message);
        return false;
    }
});

ipcMain.handle("inker:mount-drop", async function (event, user, drop, mountPoint) {
    try {
        if (!mountPoint) {
            mountPoint = path.join(os.homedir(), "FtR", user, drop);
        }

        fs.mkdirSync(mountPoint, { recursive: true });
        logToRenderer("Mounting " + user + "/" + drop + " at " + mountPoint);
        const result = await mountManager.mount(user, drop, mountPoint);

        await settings.addMount(user, drop, mountPoint);
        logToRenderer("Mount saved: " + user + "/" + drop);
        return result;
    } catch (err) {
        logToRenderer("Mount failed: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:unmount-drop", async function (event, user, drop) {
    try {
        const dropPath = user + "/" + drop;
        logToRenderer("Unmounting " + dropPath);

        if (!mountManager.isMounted(dropPath)) {
            throw new Error(dropPath + " is not currently mounted");
        }

        const result = await mountManager.unmount(dropPath);
        await settings.removeMount(dropPath);
        logToRenderer("Unmount successful: " + dropPath);
        return result;
    } catch (err) {
        logToRenderer("Unmount failed: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:get-mounts", async function () {
    try {
        const mounts = mountManager.getActiveMounts();
        logToRenderer("Returning " + mounts.length + " active mount(s)");
        return mounts;
    } catch (err) {
        logToRenderer("Get mounts error: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:get-saved-mounts", async function () {
    try {
        const mounts = await settings.getMounts();
        logToRenderer("Returning " + mounts.length + " saved mount(s)");
        return mounts;
    } catch (err) {
        logToRenderer("Get saved mounts error: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:set-auto-mount", async function (event, user, drop, enabled) {
    try {
        const dropPath = user + "/" + drop;
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
        logToRenderer("Opening path: " + localPath);
        await shell.openPath(localPath);
        return { path: localPath };
    } catch (err) {
        logToRenderer("Open path failed: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:set-autostart", async function (event, enabled) {
    try {
        app.setLoginItemSettings({
            openAtLogin: !!enabled,
            openAsHidden: true,
            path: process.execPath,
            args: []
        });
        logToRenderer("Auto-start " + (enabled ? "enabled" : "disabled"));
        return { success: true, enabled: !!enabled };
    } catch (err) {
        logToRenderer("Set auto-start error: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:get-autostart", async function () {
    try {
        const status = app.getLoginItemSettings();
        return { enabled: !!status.openAtLogin };
    } catch (err) {
        logToRenderer("Get auto-start error: " + err.message);
        return { enabled: false, error: err.message };
    }
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
    if (win) win.close();
});

app.whenReady().then(function () {
    logToRenderer("Electron app ready");
    createWindow();
    initializeApp();
});

app.on("before-quit", function () {
    logToRenderer("Application shutting down...");
    if (mountManager) mountManager.closeAll();
    if (syncManager) syncManager.stopAllSync();
    if (settings) settings.close();
});

app.on("window-all-closed", function () {
    if (process.platform !== "darwin") {
        app.quit();
    }
});