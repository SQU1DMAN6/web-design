const { app, BrowserWindow, ipcMain, shell, Tray, Menu, nativeImage } = require("electron");
const path = require("path");
const os = require("os");
const fs = require("fs");

const apiClient = require("./lib/api-client");
const MountManager = require("./lib/mount-manager");
const SyncManager = require("./lib/sync-manager");
const settings = require("./lib/settings");

let win;
let tray;
let mountManager;
let syncManager;
let appReady = false;
let isQuitting = false;

const gotTheLock = app.requestSingleInstanceLock();
if (!gotTheLock) {
    app.quit();
}

// Register inker:// protocol handler
if (process.defaultApp) {
    if (process.argv.length >= 2) {
        app.setAsDefaultProtocolClient("inker", process.execPath, [path.resolve(process.argv[1])]);
    }
} else {
    app.setAsDefaultProtocolClient("inker");
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
        // Only show if not launched for a protocol URL
        var isProtocolLaunch = false;
        for (var ai = 0; ai < process.argv.length; ai++) {
            if (typeof process.argv[ai] === "string" && process.argv[ai].startsWith("inker://")) {
                isProtocolLaunch = true;
                break;
            }
        }
        if (!isProtocolLaunch) {
            win.show();
        }
    });

    win.on("close", function (e) {
        if (!isQuitting) {
            e.preventDefault();
            win.hide();
            logToRenderer("Window hidden to tray (background sync continues)");
        }
    });
}

function createTray() {
    var iconPath = path.join(__dirname, "icon.ico");
    var trayIcon = nativeImage.createFromPath(iconPath);
    tray = new Tray(trayIcon);
    tray.setToolTip("FtR Inker");

    var contextMenu = Menu.buildFromTemplate([
        {
            label: "Open FtR Inker",
            click: function () {
                if (win) {
                    if (win.isMinimized()) win.restore();
                    win.show();
                    win.focus();
                }
            }
        },
        {
            label: "Open FtR Folder",
            click: function () {
                var ftrDir = path.join(os.homedir(), "FtR");
                if (!fs.existsSync(ftrDir)) {
                    fs.mkdirSync(ftrDir, { recursive: true });
                }
                shell.openPath(ftrDir);
            }
        },
        { type: "separator" },
        {
            label: "Exit FtR Inker",
            click: function () {
                isQuitting = true;
                app.quit();
            }
        }
    ]);

    tray.setContextMenu(contextMenu);
    tray.on("double-click", function () {
        if (win) {
            if (win.isMinimized()) win.restore();
            win.show();
            win.focus();
        }
    });
}

function logToRenderer(message) {
    if (!win || win.isDestroyed()) return;
    var ts = new Date().toISOString().replace("T", " ").slice(0, 19);
    console.log("[INKER] " + ts + " " + message);
    if (win.webContents) {
        try { win.webContents.send("inker:log", message); } catch (e) { /* ignore */ }
    }
}

// Handle inker:// protocol URLs — WITHOUT showing the window
function handleInkerUrl(url) {
    logToRenderer("Protocol URL: " + url);
    var parsed = url.replace("inker://", "").split("/");
    if (parsed.length >= 3) {
        var user = decodeURIComponent(parsed[0]);
        var drop = decodeURIComponent(parsed[1]);
        var filePath = parsed.slice(2).map(function (p) { return decodeURIComponent(p); }).join("/");
        logToRenderer("Opening remote file: " + user + "/" + drop + "/" + filePath);
        openRemoteFileAndLaunch(user, drop, filePath);
    }
}

app.on("second-instance", function (event, argv) {
    for (var ai = 0; ai < argv.length; ai++) {
        if (typeof argv[ai] === "string" && argv[ai].startsWith("inker://")) {
            handleInkerUrl(argv[ai]);
            return;
        }
    }
    // Show window if not a protocol URL
    if (win) {
        if (win.isMinimized()) win.restore();
        win.show();
        win.focus();
    }
});

app.on("open-url", function (event, url) {
    event.preventDefault();
    handleInkerUrl(url);
});

async function openRemoteFileAndLaunch(user, drop, filePath) {
    try {
        if (!mountManager) {
            logToRenderer("Cannot open file: mount manager not ready");
            return;
        }
        var localPath = await mountManager.openCachedFile(user, drop, filePath);
        logToRenderer("Opening downloaded file: " + localPath);
        await shell.openPath(localPath);
    } catch (err) {
        logToRenderer("Failed to open remote file: " + err.message);
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

        // Check saved session
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

                // Auto-mount ALL saved drops (not just auto_mount)
                var savedMounts = await settings.getMounts();
                logToRenderer("Found " + savedMounts.length + " saved mount(s)");
                for (var mi = 0; mi < savedMounts.length; mi++) {
                    var mount = savedMounts[mi];
                    try {
                        // Check if mount point exists already (might still be mounted from last session)
                        if (fs.existsSync(mount.mount_point)) {
                            // Check if index file exists to verify it was mounted here
                            var indexPath = path.join(mount.mount_point, ".ftr_index.json");
                            if (fs.existsSync(indexPath)) {
                                logToRenderer("Drop " + mount.drop_path + " appears already mounted at " + mount.mount_point);
                                // Load it into our state
                                try {
                                    var raw = fs.readFileSync(indexPath, "utf8");
                                    var data = JSON.parse(raw);
                                    mountManager.activeMounts.set(mount.drop_path, {
                                        user: mount.user,
                                        drop: mount.drop,
                                        mountPoint: mount.mount_point,
                                        syncing: false,
                                        index: data.entries || [],
                                        remoteEntries: []
                                    });
                                    syncManager.startSync(mount.drop_path, mount.mount_point);
                                    continue;
                                } catch (e) {
                                    logToRenderer("Index parse failed, re-mounting: " + e.message);
                                }
                            }
                        }
                        logToRenderer("Mounting " + mount.drop_path + " at " + mount.mount_point);
                        await mountManager.mount(mount.user, mount.drop, mount.mount_point);
                    } catch (err) {
                        logToRenderer("Failed to mount " + mount.drop_path + ": " + err.message);
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

        // Handle any inker:// protocol URL from argv
        for (var ai = 0; ai < process.argv.length; ai++) {
            if (typeof process.argv[ai] === "string" && process.argv[ai].startsWith("inker://")) {
                setTimeout(function (url) { handleInkerUrl(url); }, 1500, process.argv[ai]);
                break;
            }
        }

        logToRenderer("Application initialized successfully");
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

        var sessionResult = await apiClient.sessionConfirm();
        if (sessionResult.valid) {
            logToRenderer("Login confirmed: " + result.username);
            return { success: true, username: result.username, email: email };
        }
        return { success: true, username: result.username, email: email };
    } catch (err) {
        logToRenderer("Login failed: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:logout", async function () {
    try {
        logToRenderer("Logging out...");
        if (mountManager) mountManager.closeAll();
        if (syncManager) syncManager.stopAllSync();
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

ipcMain.handle("inker:get-file-index", async function (event, user, drop) {
    try {
        if (!mountManager) return [];
        var index = mountManager.getIndex(user + "/" + drop);
        if (!index) return [];
        return index.filter(function (e) { return e.kind !== "directory"; });
    } catch (err) {
        logToRenderer("Get file index error: " + err.message);
        return [];
    }
});

ipcMain.handle("inker:open-file", async function (event, user, drop, filePath) {
    try {
        logToRenderer("Opening file: " + user + "/" + drop + "/" + filePath);
        // Try cache first, fall back to download
        var localPath = await mountManager.openCachedFile(user, drop, filePath);
        logToRenderer("Using cached file: " + localPath);
        var result = await shell.openPath(localPath);
        logToRenderer("Shell open result: " + result);
        return { path: localPath, openResult: result };
    } catch (err) {
        logToRenderer("Open file error: " + err.message);
        throw err;
    }
});

ipcMain.handle("inker:mount-drop", async function (event, user, drop, mountPoint) {
    try {
        if (!mountPoint) {
            mountPoint = path.join(os.homedir(), "FtR", user, drop);
        }

        fs.mkdirSync(mountPoint, { recursive: true });
        logToRenderer("Adding " + user + "/" + drop + " at " + mountPoint);

        var dropPath = user + "/" + drop;
        if (mountManager.isMounted(dropPath)) {
            logToRenderer(user + "/" + drop + " already added, reloading...");
            await mountManager.unmount(dropPath);
            syncManager.stopSync(dropPath);
        }

        var result = await mountManager.mount(user, drop, mountPoint);
        // Save with auto_mount=true by default so it restores on next start
        await settings.addMount(user, drop, mountPoint, true);
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

        if (state && state.mountPoint) {
            var mp = state.mountPoint;
            logToRenderer("Cleaning up files at " + mp);
            if (fs.existsSync(mp)) {
                fs.rmSync(mp, { recursive: true, force: true });
                logToRenderer("Deleted " + mp);
                var dirs = mp.split(path.sep);
                for (var di = dirs.length - 1; di >= 2; di--) {
                    var parent = dirs.slice(0, di).join(path.sep);
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
        return mountManager ? mountManager.getActiveMounts() : [];
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
        if (!mountManager) return { size: 0, path: "" };
        var size = mountManager.getCacheSize();
        return {
            size: size,
            path: path.join(os.homedir(), ".inker", "cache")
        };
    } catch (err) {
        logToRenderer("Get cache info error: " + err.message);
        return { size: 0, path: "" };
    }
});

ipcMain.handle("inker:clear-cache", async function () {
    try {
        if (!mountManager) return { success: false };
        mountManager.clearCache();
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
    if (win) win.hide(); // Close button sends to tray
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
    if (syncManager) {
        syncManager.stopAllSync();
    }
    if (mountManager) {
        // Unmount all drops and clean up
        var mounts = mountManager.getActiveMounts();
        mounts.forEach(function (m) {
            var mp = m.mountPoint;
            try {
                if (fs.existsSync(mp)) {
                    fs.rmSync(mp, { recursive: true, force: true });
                    logToRenderer("Cleaned up " + mp);
                }
            } catch (e) {
                logToRenderer("Cleanup error: " + e.message);
            }
        });
        mountManager.closeAll();
    }
    if (settings) settings.close();
    if (tray) tray.destroy();
});

app.on("window-all-closed", function () {
    // Don't quit — we have a tray
});