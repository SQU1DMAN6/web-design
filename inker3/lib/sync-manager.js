const fs = require("fs");
const path = require("path");
const { EventEmitter } = require("events");
const chokidar = require("chokidar");

class SyncManager extends EventEmitter {
    constructor(apiClient) {
        super();
        this.apiClient = apiClient;
        this.activeWatchers = new Map();
        this.fileSyncState = new Map();
        this.syncInterval = 5000;
    }

    startSync(dropPath, mountPoint) {
        if (this.activeWatchers.has(dropPath)) {
            return;
        }

        var parts = dropPath.split("/");
        var user = parts[0];
        var drop = parts[1];

        var watcher = chokidar.watch(mountPoint, {
            ignored: /(^|[/\\])\..*|node_modules/,
            persistent: true,
            ignoreInitial: true,
            awaitWriteFinish: { stabilityThreshold: 500, pollInterval: 100 }
        });

        var self = this;

        watcher.on("add", function (filePath) {
            var rel = path.relative(mountPoint, filePath).replace(/\\/g, "/");
            self._handleFileChange(user, drop, dropPath, rel, "add", filePath);
        });

        watcher.on("change", function (filePath) {
            var rel = path.relative(mountPoint, filePath).replace(/\\/g, "/");
            self._handleFileChange(user, drop, dropPath, rel, "change", filePath);
        });

        watcher.on("unlink", function (filePath) {
            var rel = path.relative(mountPoint, filePath).replace(/\\/g, "/");
            self._handleFileDelete(user, drop, dropPath, rel);
        });

        this.activeWatchers.set(dropPath, watcher);
        console.log("[SyncManager] Watching " + dropPath + " at " + mountPoint);
    }

    async _handleFileChange(user, drop, dropPath, relPath, type, fullPath) {
        try {
            if (relPath.includes("/") || relPath.includes("\\")) {
                var dir = path.dirname(relPath);
                try {
                    await this.apiClient.createDirectory(user, drop, dir);
                } catch (e) {
                    // Directory may already exist
                }
            }

            var stat = fs.statSync(fullPath);
            if (!stat.isFile()) return;

            var fileStream = fs.createReadStream(fullPath);
            await this.apiClient.uploadFile(user, drop, relPath, fileStream, stat.size);
            this.emit("file-synced", { dropPath: dropPath, file: relPath, type: type });
        } catch (err) {
            this.emit("sync-error", { dropPath: dropPath, file: relPath, error: err.message });
        }
    }

    async _handleFileDelete(user, drop, dropPath, relPath) {
        try {
            await this.apiClient.deleteFile(user, drop, relPath);
            this.emit("file-synced", { dropPath: dropPath, file: relPath, type: "delete" });
        } catch (err) {
            this.emit("sync-error", { dropPath: dropPath, file: relPath, error: err.message });
        }
    }

    stopSync(dropPath) {
        var watcher = this.activeWatchers.get(dropPath);
        if (watcher) {
            watcher.close();
            this.activeWatchers.delete(dropPath);
            console.log("[SyncManager] Stopped watching " + dropPath);
        }
    }

    stopAllSync() {
        this.activeWatchers.forEach(function (watcher, dropPath) {
            watcher.close();
        });
        this.activeWatchers.clear();
        console.log("[SyncManager] Stopped all watchers");
    }
}

module.exports = SyncManager;