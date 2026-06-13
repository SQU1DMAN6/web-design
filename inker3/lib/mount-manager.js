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

    _shouldInclude(relPath) {
        var parts = relPath.split("/");
        for (var pi = 0; pi < parts.length; pi++) {
            if (parts[pi].startsWith(".")) return false;
        }
        if (relPath.endsWith(".url") || relPath.endsWith(".lnk") || relPath.endsWith(".ftr_index.json")) return false;
        return true;
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
            // Skip hidden/dotfiles in the index, but NOT .url files (those are local only)
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
                // Create .url shortcut file (Windows Internet Shortcut)
                // These are LOCAL ONLY — never synced to remote (sync-manager filters .url)
                var parentDir = path.join(mountPoint, path.dirname(relPath));
                fs.mkdirSync(parentDir, { recursive: true });
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

        console.log("[MountManager] " + dropPath + " added with " + indexEntries.length + " entries (" + (entries.length - indexEntries.length) + " filtered)");
        this.emit("mounted", { dropPath: dropPath, mountPoint: mountPoint });
        return { dropPath: dropPath, mountPoint: mountPoint };
    }

    async openRemoteFile(user, drop, relPath) {
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

    async openCachedFile(user, drop, relPath) {
        var cacheDir = path.join(os.homedir(), ".inker", "cache", user, drop);
        var fileName = relPath.split("/").pop();
        var localPath = path.join(cacheDir, fileName);

        if (fs.existsSync(localPath)) {
            console.log("[MountManager] Cache hit: " + localPath);
            return localPath;
        }
        return await this.openRemoteFile(user, drop, relPath);
    }

    getCacheSize() {
        var cacheRoot = path.join(os.homedir(), ".inker", "cache");
        if (!fs.existsSync(cacheRoot)) return 0;
        var total = 0;
        try {
            this._walkSize(cacheRoot, function(size) { total += size; });
        } catch (e) {}
        return total;
    }

    _walkSize(dir, callback) {
        var entries = fs.readdirSync(dir, { withFileTypes: true });
        for (var i = 0; i < entries.length; i++) {
            var e = entries[i];
            var full = path.join(dir, e.name);
            if (e.isDirectory()) {
                this._walkSize(full, callback);
            } else if (e.isFile()) {
                try { callback(fs.statSync(full).size); } catch (ex) {}
            }
        }
    }

    clearCache() {
        var cacheRoot = path.join(os.homedir(), ".inker", "cache");
        if (fs.existsSync(cacheRoot)) {
            fs.rmSync(cacheRoot, { recursive: true, force: true });
            fs.mkdirSync(cacheRoot, { recursive: true });
        }
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