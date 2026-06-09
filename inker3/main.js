const { app, BrowserWindow, ipcMain, shell, dialog } = require("electron");
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

// Make sure only one instance of Inker runs at a time (daemon behavior).
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
    console.log(`[INKER] ${message}`);
    if (win && win.webContents) {
        win.webContents.send("inker:log", message);
    }
}

async function initializeApp() {
    try {
        mountManager = new MountManager(apiClient);
        syncManager = new SyncManager(apiClient);

        mountManager.on("mounted", ({ repoPath, mountPoint }) => {
            logToRenderer(`✓ Mounted ${repoPath} at ${mountPoint}`);
            syncManager.startSync(repoPath, mountPoint);
        });

        mountManager.on("unmounted", ({ repoPath }) => {
            logToRenderer(`✓ Unmounted ${repoPath}`);
            syncManager.stopSync(repoPath);
        });

        syncManager.on("file-synced", ({ repoPath, file, type }) => {
            logToRenderer(`[sync] ${type.toUpperCase()} ${repoPath}/${file}`);
        });

        syncManager.on("sync-error", ({ repoPath, file, error }) => {
            logToRenderer(`[sync error] ${repoPath}/${file}: ${error}`);
        });

        const auth = await settings.getAuth();
        if (auth) {
            apiClient.setSession(auth.email, auth.username, auth.session_id);
            logToRenderer(`Loaded session for ${auth.username}`);

            const mounts = await settings.getMounts();
            for (const mount of mounts) {
                if (mount.auto_mount) {
                    try {
                        logToRenderer(`Auto-mounting ${mount.repo_path} at ${mount.mount_point}`);
                        await mountManager.mount(mount.user, mount.repo, mount.mount_point);
                    } catch (err) {
                        logToRenderer(`Failed to auto-mount ${mount.repo_path}: ${err.message}`);
                    }
                }
            }
        }

        appReady = true;
        if (win) win.webContents.send("inker:ready");
    } catch (err) {
        logToRenderer(`Initialization error: ${err.message}`);
    }
}

ipcMain.handle("inker:login", async (event, email, password) => {
    try {
        logToRenderer(`Attempting login for ${email}`);
        const result = await apiClient.login(email, password);
        await settings.saveAuth(email, result.username, apiClient.sessionId);
        logToRenderer(`✓ Logged in as ${result.username}`);
        return { success: true, ...result };
    } catch (err) {
        logToRenderer(`Login failed: ${err.message}`);
        throw err;
    }
});

ipcMain.handle("inker:logout", async () => {
    try {
        logToRenderer("Logging out...");
        mountManager.closeAll();
        syncManager.stopAllSync();
        await settings.clearAuth();
        apiClient.sessionId = null;
        apiClient.email = null;
        apiClient.username = null;
        logToRenderer("✓ Logged out");
        return { success: true };
    } catch (err) {
        logToRenderer(`Logout error: ${err.message}`);
        throw err;
    }
});

ipcMain.handle("inker:get-current-user", async () => {
    const auth = await settings.getAuth();
    if (auth) {
        return { email: auth.email, username: auth.username };
    }
    return null;
});

ipcMain.handle("inker:search-repos", async (event, query) => {
    try {
        logToRenderer(`Searching for "${query}"`);
        const results = await apiClient.searchRepositories(query);
        logToRenderer(`Found ${results.length} repositories`);
        return results;
    } catch (err) {
        logToRenderer(`Search failed: ${err.message}`);
        throw err;
    }
});

ipcMain.handle("inker:list-repos", async () => {
    try {
        const results = await apiClient.listRepositories();
        return results;
    } catch (err) {
        logToRenderer(`List repositories failed: ${err.message}`);
        throw err;
    }
});

ipcMain.handle("inker:mount-repo", async (event, user, repo, mountPoint) => {
    try {
        if (!mountPoint) {
            mountPoint = path.join(os.homedir(), "FtR", user, repo);
        }

        fs.mkdirSync(mountPoint, { recursive: true });

        logToRenderer(`Mounting ${user}/${repo} at ${mountPoint}`);
        const result = await mountManager.mount(user, repo, mountPoint);

        await settings.addMount(user, repo, mountPoint);
        return result;
    } catch (err) {
        logToRenderer(`Mount failed: ${err.message}`);
        throw err;
    }
});

ipcMain.handle("inker:unmount-repo", async (event, user, repo) => {
    try {
        const repoPath = `${user}/${repo}`;
        logToRenderer(`Unmounting ${repoPath}`);

        if (!mountManager.isMounted(repoPath)) {
            throw new Error(`${repoPath} is not currently mounted`);
        }

        const result = await mountManager.unmount(repoPath);
        await settings.removeMount(repoPath);
        return result;
    } catch (err) {
        logToRenderer(`Unmount failed: ${err.message}`);
        throw err;
    }
});

ipcMain.handle("inker:get-mounts", async () => {
    try {
        return mountManager.getActiveMounts();
    } catch (err) {
        logToRenderer(`Get mounts error: ${err.message}`);
        throw err;
    }
});

ipcMain.handle("inker:get-saved-mounts", async () => {
    try {
        return await settings.getMounts();
    } catch (err) {
        logToRenderer(`Get saved mounts error: ${err.message}`);
        throw err;
    }
});

ipcMain.handle("inker:set-auto-mount", async (event, user, repo, enabled) => {
    try {
        const repoPath = `${user}/${repo}`;
        await settings.setAutoMount(repoPath, enabled);
        logToRenderer(`Auto-mount ${enabled ? "enabled" : "disabled"} for ${repoPath}`);
        return { success: true };
    } catch (err) {
        logToRenderer(`Set auto-mount error: ${err.message}`);
        throw err;
    }
});

ipcMain.handle("inker:open-path", async (event, localPath) => {
    try {
        await shell.openPath(localPath);
        return { path: localPath };
    } catch (err) {
        throw err;
    }
});

// Windows auto-start: register/unregister this app to launch at user logon.
ipcMain.handle("inker:set-autostart", async (event, enabled) => {
    try {
        app.setLoginItemSettings({
            openAtLogin: !!enabled,
            openAsHidden: true,
            path: process.execPath,
            args: []
        });
        logToRenderer(`Auto-start ${enabled ? "enabled" : "disabled"} (login item updated)`);
        return { success: true, enabled: !!enabled };
    } catch (err) {
        logToRenderer(`Set auto-start error: ${err.message}`);
        throw err;
    }
});

ipcMain.handle("inker:get-autostart", async () => {
    try {
        const status = app.getLoginItemSettings();
        return { enabled: !!status.openAtLogin };
    } catch (err) {
        return { enabled: false, error: err.message };
    }
});

ipcMain.on("window:minimize", () => {
    if (win) win.minimize();
});

ipcMain.on("window:maximize", () => {
    if (win) {
        if (win.isMaximized()) {
            win.unmaximize();
        } else {
            win.maximize();
        }
    }
});

ipcMain.on("window:close", () => {
    if (win) win.close();
});

app.whenReady().then(() => {
    createWindow();
    initializeApp();
});

app.on("before-quit", () => {
    if (mountManager) mountManager.closeAll();
    if (syncManager) syncManager.stopAllSync();
    if (settings) settings.close();
});

app.on("window-all-closed", () => {
    if (process.platform !== "darwin") {
        app.quit();
    }
});

