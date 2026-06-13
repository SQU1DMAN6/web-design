const fs = require("fs");
const path = require("path");
const os = require("os");
const { EventEmitter } = require("events");

class MountManager extends EventEmitter {
    constructor(apiClient) {
        super();
        this.apiClient = apiClient;
        this.activeMounts = new Map();
    }

    async mount(user, drop, mountPoint) {
        var dropPath = user + "/" + drop;
        console.log("[MountManager] mount ENTER: " + dropPath + " -> " + mountPoint);

        if (this.activeMounts.has(dropPath)) {
            console.log("[MountManager] mount ERROR: " + dropPath + " is already added");
            throw new Error(dropPath + " is already added");
        }

        fs.mkdirSync(mountPoint, { recursive: true });

        var entries = [];
        try {
            console.log("[MountManager] getFileList for " + dropPath);
            entries = await this.apiClient.getFileList(user, drop);
            console.log("[MountManager] getFileList returned " + entries.length + " entries");
        } catch (err) {
            console.log("[MountManager] getFileList failed: " + err.message);
        }

        var indexEntries = [];
        for (var i = 0; i < entries.length; i++) {
            var entry = entries[i];
            var relPath = entry.path || entry.name;
            if (!relPath) continue;

            // Skip hidden/trash entries (any component starting with .)
            var nameParts = relPath.split("/");
            var skip = false;
            for (var pi = 0; pi < nameParts.length; pi++) {
                if (nameParts[pi].startsWith(".")) {
                    skip = true;
                    break;
                }
            }
            if (skip) continue;

            var isDir = entry.kind === "directory" || entry.type === "dir" || relPath.endsWith("/");
            var originalName = nameParts[nameParts.length - 1];

            if (isDir) {
                var localDir = path.join(mountPoint, relPath);
                fs.mkdirSync(localDir, { recursive: true });
            } else {
                var parentDir = path.join(mountPoint, path.dirname(relPath));
                fs.mkdirSync(parentDir, { recursive: true });

                // Create .url shortcut file (Windows Internet Shortcut format)
                // Format: [InternetShortcut]\r\nURL=inker://user/drop/encoded-path
                var encodedPath = nameParts.map(function(p) { return encodeURIComponent(p); }).join("/");
                var inkerUrl = "inker://" + user + "/" + drop + "/" + encodedPath;
                var shortcutPath = path.join(mountPoint, relPath + ".url");
                if (!fs.existsSync(shortcutPath)) {
                    fs.writeFileSync(shortcutPath, "[InternetShortcut]\r\nURL=" + inkerUrl + "\r\n");
                }
            }

            indexEntries.push({
                path: relPath,
                name: originalName,
                size: entry.size || 0,
                modified: entry.modified || entry.mtime || "",
                kind: isDir ? "directory" : "file",
                synced: false,
                lastAccess: 0
            });
        }

        // Write metadata index
        var indexPath = path.join(mountPoint, ".ftr_index.json");
        fs.writeFileSync(indexPath, JSON.stringify({
            user: user,
            drop: drop,
            mountPoint: mountPoint,
            entries: indexEntries
        }, null, 2));

        this.activeMounts.set(dropPath, {
            user: user,
            drop: drop,
            mountPoint: mountPoint,
            syncing: false,
            index: indexEntries,
            remoteEntries: entries
        });

        console.log("[MountManager] " + dropPath + " added with " + indexEntries.length + " entries (" + (entries.length - indexEntries.length) + " hidden skipped)");
        this.emit("mounted", { dropPath: dropPath, mountPoint: mountPoint });
        return { dropPath: dropPath, mountPoint: mountPoint };
    }

    async openRemoteFile(user, drop, relPath) {
        // Download a file and return the local cached path
        console.log("[MountManager] openRemoteFile: " + user + "/" + drop + "/" + relPath);

        var cacheDir = path.join(os.homedir(), ".inker", "cache", user, drop);
        fs.mkdirSync(cacheDir, { recursive: true });

        var fileName = relPath.split("/").pop();
        var localPath = path.join(cacheDir, fileName);

        var stream = await this.apiClient.downloadFile(user, drop, relPath);
        await new Promise(function (resolve, reject) {
            var out = fs.createWriteStream(localPath);
            stream.on("error", reject);
            out.on("error", reject);
            out.on("finish", resolve);
            stream.pipe(out);
        });

        console.log("[MountManager] Downloaded to: " + localPath);
        return localPath;
    }

    async _downloadFile(user, drop, relPath, mountPoint) {
        console.log("[MountManager] _downloadFile: " + relPath);
        var localPath = path.join(mountPoint, relPath);
        var localDir = path.dirname(localPath);
        fs.mkdirSync(localDir, { recursive: true });

        var stream = await this.apiClient.downloadFile(user, drop, relPath);
        return new Promise(function (resolve, reject) {
            var out = fs.createWriteStream(localPath);
            stream.on("error", reject);
            out.on("error", reject);
            out.on("finish", function () {
                console.log("[MountManager] _downloadFile saved: " + relPath);
                resolve(localPath);
            });
            stream.pipe(out);
        });
    }

    async _uploadFile(user, drop, relPath, mountPoint) {
        console.log("[MountManager] _uploadFile: " + relPath);
        var localPath = path.join(mountPoint, relPath);
        if (!fs.existsSync(localPath)) return;

        var content = fs.readFileSync(localPath);
        await this.apiClient.uploadFile(user, drop, relPath, content);
        console.log("[MountManager] _uploadFile uploaded: " + relPath);
    }

    async unmount(dropPath) {
        console.log("[MountManager] unmount ENTER: " + dropPath);
        if (!this.activeMounts.has(dropPath)) {
            throw new Error(dropPath + " is not added");
        }
        var state = this.activeMounts.get(dropPath);
        this.activeMounts.delete(dropPath);
        this.emit("unmounted", { dropPath: dropPath, mountPoint: state.mountPoint });
        return { dropPath: dropPath, mountPoint: state.mountPoint };
    }

    getActiveMounts() {
        var result = [];
        this.activeMounts.forEach(function (state, dropPath) {
            result.push({ dropPath: dropPath, mountPoint: state.mountPoint, syncing: state.syncing });
        });
        return result;
    }

    isMounted(dropPath) {
        return this.activeMounts.has(dropPath);
    }

    getMount(dropPath) {
        return this.activeMounts.get(dropPath);
    }

    getIndex(dropPath) {
        var state = this.activeMounts.get(dropPath);
        if (!state) return null;
        return state.index || [];
    }

    updateFileAccess(dropPath, relPath) {
        var state = this.activeMounts.get(dropPath);
        if (!state) return;
        var entries = state.index;
        for (var i = 0; i < entries.length; i++) {
            if (entries[i].path === relPath) {
                entries[i].lastAccess = Date.now();
                entries[i].synced = true;
                break;
            }
        }
    }

    closeAll() {
        this.activeMounts.clear();
    }
}

module.exports = MountManager;