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
let isCLI = false;

const gotTheLock = app.requestSingleInstanceLock();
if (!gotTheLock) {
    app.quit();
}

async function cliMount(user, drop) {
    isCLI = true;
    var mountPoint = path.join(os.homedir(), "FtR", user, drop);
    console.log("[Inker CLI] Adding " + user + "/" + drop + " at " + mountPoint);

    try {
        var auth = await settings.getAuth();
        if (auth) {
            apiClient.setSession(auth.email, auth.username, auth.session_id);
        } else {
            console.error("[Inker CLI] Not logged in. Please log in first.");
            app.quit();
            return;
        }

        var exists = await apiClient.verifyDrop(user, drop);
        if (!exists) {
            console.error("[Inker CLI] Drop " + user + "/" + drop + " does not exist.");
            app.quit();
            return;
        }

        mountManager = new MountManager(apiClient);
        syncManager = new SyncManager(apiClient);

        mountManager.on("mounted", function (data) {
            console.log("[Inker CLI] Added " + data.dropPath + " at " + data.mountPoint);
            syncManager.startSync(data.dropPath, data.mountPoint);
            console.log("[Inker CLI] Syncing " + data.dropPath + "...");
        });

        await mountManager.mount(user, drop, mountPoint);
        await settings.addMount(user, drop, mountPoint);

        console.log("[Inker CLI] Opening " + mountPoint);
        await shell.openPath(mountPoint);

        console.log("[Inker CLI] " + user + "/" + drop + " is ready. Closing in 3 seconds...");
        setTimeout(function () {
            mountManager.closeAll();
            syncManager.stopAllSync();
            settings.close();
            app.quit();
        }, 3000);
    } catch (err) {
        console.error("[Inker CLI] Error: " + err.message);
        app.quit();
    }
}

var args = process.argv.slice(1);
if (args.length > 1 && args[1] === "mount") {
    if (args.length > 2) {
        var parts = args[2].split("/");
        if (parts.length === 2 && parts[0] && parts[1]) {
            app.whenReady().then(function () {
                cliMount(parts[0], parts[1]);
            });
            return;
        }
    }
    console.error("Usage: inker mount user/dropname");
    app.quit();
    return;
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
    if (!win || win.isDestroyed()) return;
    var ts = new Date().toISOString().replace("T", " ").slice(0, 19);
    console.log("[INKER] " + ts + " " + message);
    if (win.webContents) {
        try { win.webContents.send("inker:log", message); } catch (e) { /* ignore */ }
    }
}

async function initializeApp() {
    logToRenderer("Initializing application...");
    try {
        mountManager = new MountManager(apiClient);
        syncManager = new SyncManager(apiClient);

        mountManager.on("mounted", function (data) {
            logToRenderer("Added " + data.dropPath + " at " + data.mountPoint);
            syncManager.startSync(data.dropPath, data.mountPoint);
        });

        mountManager.on("unmounted", function (data) {
            logToRenderer("Removed " + data.dropPath);
            syncManager.stopSync(data.dropPath);
        });

        syncManager.on("file-synced", function (data) {
            logToRenderer("[sync] " + data.type.toUpperCase() + " " + data.dropPath + "/" + data.file);
        });

        syncManager.on("sync-error", function (data) {
            logToRenderer("[sync error] " + data.dropPath + "/" + data.file + ": " + data.error);
        });

        var auth = await settings.getAuth();
        if (auth) {
            apiClient.setSession(auth.email, auth.username, auth.session_id);
            logToRenderer("Loaded session for " + auth.username);

            // Enable auto-start on first run
            try {
                if (!app.getLoginItemSettings().openAtLogin) {
                    app.setLoginItemSettings({ openAtLogin: true, openAsHidden: true, path: process.execPath });
                    logToRenderer("Auto-start enabled");
                }
            } catch (e) {
                logToRenderer("Auto-start check: " + e.message);
            }

            var mounts = await settings.getMounts();
            logToRenderer("Found " + mounts.length + " saved mount(s)");
            for (var mi = 0; mi < mounts.length; mi++) {
                var mount = mounts[mi];
                if (mount.auto_mount) {
                    try {
                        logToRenderer("Auto-adding " + mount.drop_path + " at " + mount.mount_point);
                        await mountManager.mount(mount.user, mount.drop, mount.mount_point);
                    } catch (err) {
                        logToRenderer("Failed to auto-add " + mount.drop_path + ": " + err.message);
                    }
                }
            }
        } else {
            logToRenderer("No saved session found");
        }

        appReady = true;
        if (win && !win.isDestroyed()) win.webContents.send("inker:ready");
        logToRenderer("Application initialized successfully");
    } catch (err) {
        logToRenderer("Initialization error: " + err.message);
    }
}

ipcMain.handle("inker:login", async function (event, email, password) {
    try {
        logToRenderer("Attempting login for " + email);
        var result = await apiClient.login(email, password);
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
    var auth = await settings.getAuth();
    if (auth) {
        return { email: auth.email, username: auth.username };
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

ipcMain.handle("inker:list-drops", async function () {
    try {
        logToRenderer("Listing all Drops...");
        var results = await apiClient.listDrops();
        logToRenderer("Found " + results.length + " Drop(s)");
        return results;
    } catch (err) {
        logToRenderer("List Drops failed: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:verify-drop", async function (event, user, drop) {
    try {
        logToRenderer("Checking " + user + "/" + drop + "...");
        var result = await apiClient.verifyDrop(user, drop);
        logToRenderer(user + "/" + drop + ": " + (result ? "exists" : "not found"));
        return result;
    } catch (err) {
        logToRenderer("Check failed: " + err.message);
        return false;
    }
});

ipcMain.handle("inker:mount-drop", async function (event, user, drop, mountPoint) {
    try {
        if (!mountPoint) {
            mountPoint = path.join(os.homedir(), "FtR", user, drop);
        }

        fs.mkdirSync(mountPoint, { recursive: true });
        logToRenderer("Adding " + user + "/" + drop + " at " + mountPoint);

        var existingPath = user + "/" + drop;
        if (mountManager.isMounted(existingPath)) {
            logToRenderer(user + "/" + drop + " already added, reloading...");
            await mountManager.unmount(existingPath);
            syncManager.stopSync(existingPath);
        }

        var result = await mountManager.mount(user, drop, mountPoint);
        await settings.addMount(user, drop, mountPoint);
        logToRenderer("Added " + user + "/" + drop);
        return result;
    } catch (err) {
        logToRenderer("Add failed: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:unmount-drop", async function (event, user, drop) {
    try {
        var dropPath = user + "/" + drop;
        logToRenderer("Removing " + dropPath);

        if (!mountManager.isMounted(dropPath)) {
            throw new Error(dropPath + " is not currently added");
        }

        var state = mountManager.getMount(dropPath);
        var result = await mountManager.unmount(dropPath);
        syncManager.stopSync(dropPath);
        await settings.removeMount(dropPath);

        // Clean up files on disk
        if (state && state.mountPoint) {
            var mp = state.mountPoint;
            logToRenderer("Cleaning up files at " + mp);
            if (fs.existsSync(mp)) {
                fs.rmSync(mp, { recursive: true, force: true });
                logToRenderer("Deleted " + mp);
                // Remove empty parent directories up the chain
                var dirs = mp.split(path.sep);
                for (var i = dirs.length - 1; i >= 2; i--) {
                    var parent = dirs.slice(0, i).join(path.sep);
                    if (parent.length < 4) break;
                    try {
                        if (fs.existsSync(parent) && fs.readdirSync(parent).length === 0) {
                            fs.rmdirSync(parent);
                            logToRenderer("Removed empty parent: " + parent);
                        } else {
                            break;
                        }
                    } catch (e) { break; }
                }
            }
        }

        logToRenderer("Removed " + dropPath);
        return result;
    } catch (err) {
        logToRenderer("Remove failed: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:get-mounts", async function () {
    try {
        var mounts = mountManager.getActiveMounts();
        return mounts;
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
        logToRenderer("Auto add " + (enabled ? "enabled" : "disabled") + " for " + dropPath);
        return { success: true };
    } catch (err) {
        logToRenderer("Set auto add error: " + err.message);
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

ipcMain.handle("inker:get-index", function (event, user, drop) {
    var dropPath = user + "/" + drop;
    var index = mountManager.getIndex(dropPath);
    if (!index) return [];
    return index;
});

ipcMain.handle("inker:download-file", async function (event, user, drop, relPath) {
    logToRenderer("Downloading " + user + "/" + drop + "/" + relPath);
    var state = mountManager.getMount(user + "/" + drop);
    if (!state) throw new Error("Drop not added");
    var localPath = await mountManager._downloadFile(user, drop, relPath, state.mountPoint);
    mountManager.updateFileAccess(user + "/" + drop, relPath);
    logToRenderer("Downloaded " + relPath + " -> " + localPath);
    return { path: localPath };
});

ipcMain.handle("inker:upload-file", async function (event, user, drop, relPath) {
    logToRenderer("Uploading " + user + "/" + drop + "/" + relPath);
    var state = mountManager.getMount(user + "/" + drop);
    if (!state) throw new Error("Drop not added");
    await mountManager._uploadFile(user, drop, relPath, state.mountPoint);
    logToRenderer("Uploaded " + relPath);
    return { success: true };
});

ipcMain.handle("inker:evict-files", async function (event, user, drop) {
    var state = mountManager.getMount(user + "/" + drop);
    if (!state) return { evicted: 0 };
    var index = state.index;
    var cutoff = Date.now() - (15 * 60 * 1000);
    var evicted = 0;
    for (var i = 0; i < index.length; i++) {
        var entry = index[i];
        if (entry.kind === "directory") continue;
        if (!entry.synced) continue;
        if (entry.lastAccess > 0 && entry.lastAccess < cutoff) {
            var localPath = path.join(state.mountPoint, entry.path);
            if (fs.existsSync(localPath)) {
                try { fs.unlinkSync(localPath); evicted++; } catch (e) {}
            }
            entry.synced = false;
            entry.lastAccess = 0;
        }
    }
    logToRenderer("Evicted " + evicted + " files from " + user + "/" + drop);
    return { evicted: evicted };
});

app.whenReady().then(function () {
    logToRenderer("Electron app ready");
    createWindow();
    initializeApp();
});

app.on("before-quit", function () {
    if (!win || win.isDestroyed()) return;
    if (mountManager) mountManager.closeAll();
    if (syncManager) syncManager.stopAllSync();
    if (settings) settings.close();
});

app.on("window-all-closed", function () {
    if (process.platform !== "darwin") {
        app.quit();
    }
});