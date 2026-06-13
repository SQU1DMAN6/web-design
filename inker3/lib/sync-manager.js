const fs = require("fs");
const path = require("path");
const { EventEmitter } = require("events");

class SyncManager extends EventEmitter {
    constructor(apiClient) {
        super();
        this.apiClient = apiClient;
        this.activeSyncs = new Map();
        this.syncInterval = 30000; // Poll remote every 30s
        this.localCheckInterval = 5000; // Check local changes every 5s
    }

    startSync(dropPath, mountPoint) {
        if (this.activeSyncs.has(dropPath)) return;

        var parts = dropPath.split("/");
        var user = parts[0];
        var drop = parts[1];

        var state = {
            user: user,
            drop: drop,
            dropPath: dropPath,
            mountPoint: mountPoint,
            running: true,
            syncTimer: null,
            localTimer: null,
            knownFiles: new Map() // relPath -> { mtime, size }
        };

        // Load known files from the index on disk
        this._loadIndex(state);

        // Start periodic remote sync
        state.syncTimer = setInterval(function(self, s) {
            self._syncRemote(s);
        }, this.syncInterval, this, state);

        // Start periodic local check
        state.localTimer = setInterval(function(self, s) {
            self._checkLocal(s);
        }, this.localCheckInterval, this, state);

        this.activeSyncs.set(dropPath, state);
        console.log("[SyncManager] Sync started for " + dropPath + " at " + mountPoint + " (interval=" + this.syncInterval + "ms)");
    }

    _loadIndex(state) {
        var indexPath = path.join(state.mountPoint, ".ftr_index.json");
        try {
            if (fs.existsSync(indexPath)) {
                var raw = fs.readFileSync(indexPath, "utf8");
                var data = JSON.parse(raw);
                if (data.entries && Array.isArray(data.entries)) {
                    for (var i = 0; i < data.entries.length; i++) {
                        var e = data.entries[i];
                        state.knownFiles.set(e.path, {
                            mtime: e.modified || "",
                            size: e.size || 0,
                            kind: e.kind || "file",
                            synced: e.synced || false
                        });
                    }
                }
            }
        } catch (err) {
            console.log("[SyncManager] _loadIndex error: " + err.message);
        }
    }

    _saveIndex(state) {
        var indexPath = path.join(state.mountPoint, ".ftr_index.json");
        try {
            var entries = [];
            state.knownFiles.forEach(function(info, relPath) {
                entries.push({
                    path: relPath,
                    name: relPath.split("/").pop(),
                    size: info.size,
                    modified: info.mtime,
                    kind: info.kind,
                    synced: info.synced
                });
            });
            fs.writeFileSync(indexPath, JSON.stringify({ entries: entries }, null, 2));
        } catch (err) {
            console.log("[SyncManager] _saveIndex error: " + err.message);
        }
    }

    async _syncRemote(state) {
        if (!state.running) return;
        try {
            var entries = await this.apiClient.getFileList(state.user, state.drop);
            if (!entries || entries.length === 0) return;

            var changed = false;
            for (var i = 0; i < entries.length; i++) {
                var entry = entries[i];
                var relPath = entry.path || entry.name;
                if (!relPath) continue;
                if (relPath.startsWith(".ftr_")) continue; // Skip our metadata

                var isDir = entry.kind === "directory" || entry.type === "dir" || relPath.endsWith("/");
                var known = state.knownFiles.get(relPath);
                var remoteMtime = entry.modified || entry.mtime || "";
                var remoteSize = entry.size || 0;

                if (!known) {
                    // New remote file - create stub
                    var localPath = path.join(state.mountPoint, relPath);
                    if (isDir) {
                        fs.mkdirSync(localPath, { recursive: true });
                    } else {
                        fs.mkdirSync(path.dirname(localPath), { recursive: true });
                        // Create stub file if it doesn't exist
                        if (!fs.existsSync(localPath)) {
                            fs.writeFileSync(localPath, "");
                        }
                    }
                    state.knownFiles.set(relPath, {
                        mtime: remoteMtime,
                        size: remoteSize,
                        kind: isDir ? "directory" : "file",
                        synced: false
                    });
                    changed = true;
                    this.emit("file-synced", { dropPath: state.dropPath, file: relPath, type: "remote-add" });
                } else if (!isDir && remoteMtime !== known.mtime) {
                    // Remote file changed - update stub if not locally modified
                    var localPath = path.join(state.mountPoint, relPath);
                    var localMtime = "";
                    try {
                        if (fs.existsSync(localPath)) {
                            var stat = fs.statSync(localPath);
                            localMtime = stat.mtime.toISOString();
                        }
                    } catch (e) {}

                    // Only update if local file hasn't been modified (is still a stub or unmodified)
                    if (!localMtime || known.synced === false || localMtime <= remoteMtime) {
                        // Update known info, file will be re-downloaded on access
                        known.mtime = remoteMtime;
                        known.size = remoteSize;
                        known.synced = false;
                        changed = true;
                        this.emit("file-synced", { dropPath: state.dropPath, file: relPath, type: "remote-change" });
                    }
                }
            }

            if (changed) {
                this._saveIndex(state);
            }
        } catch (err) {
            this.emit("sync-error", { dropPath: state.dropPath, file: "(remote poll)", error: err.message });
        }
    }

    async _checkLocal(state) {
        if (!state.running) return;
        try {
            var self = this;
            var mountPoint = state.mountPoint;

            // Walk the mount point recursively (but not too deep/fast)
            this._walkDir(mountPoint, mountPoint, state, function(relPath, localStat) {
                var known = state.knownFiles.get(relPath);
                var localMtime = localStat.mtime.toISOString();
                var localSize = localStat.size;

                if (!known) {
                    // New local file - upload it
                    state.knownFiles.set(relPath, {
                        mtime: localMtime,
                        size: localSize,
                        kind: "file",
                        synced: true
                    });
                    self._uploadLocalFile(state, relPath);
                } else if (localMtime !== known.mtime && known.synced) {
                    // File changed locally - upload
                    known.mtime = localMtime;
                    known.size = localSize;
                    known.synced = true;
                    self._uploadLocalFile(state, relPath);
                } else if (localMtime !== known.mtime && !known.synced) {
                    // File was downloaded (synced from remote) - update known mtime
                    known.mtime = localMtime;
                    known.size = localSize;
                    known.synced = true;
                }
            });
        } catch (err) {
            this.emit("sync-error", { dropPath: state.dropPath, file: "(local check)", error: err.message });
        }
    }

    _walkDir(basePath, currentPath, state, callback) {
        try {
            var entries = fs.readdirSync(currentPath, { withFileTypes: true });
            for (var i = 0; i < entries.length; i++) {
                var entry = entries[i];
                if (entry.name.startsWith(".ftr_")) continue;
                if (entry.name === "." || entry.name === "..") continue;

                var fullPath = path.join(currentPath, entry.name);
                var relPath = path.relative(basePath, fullPath).replace(/\\/g, "/");

                if (entry.isDirectory()) {
                    callback(relPath, { mtime: entry.mtime || new Date(0), size: 0, isDirectory: function() { return true; } });
                    this._walkDir(basePath, fullPath, state, callback);
                } else if (entry.isFile()) {
                    try {
                        var stat = fs.statSync(fullPath);
                        if (stat.size > 0) { // Only report non-stub files
                            callback(relPath, stat);
                        }
                    } catch (e) {}
                }
            }
        } catch (err) {
            // Path may not exist
        }
    }

    async _uploadLocalFile(state, relPath) {
        try {
            var fullPath = path.join(state.mountPoint, relPath);
            if (!fs.existsSync(fullPath)) return;
            var stat = fs.statSync(fullPath);
            if (!stat.isFile() || stat.size === 0) return;

            var fileStream = fs.createReadStream(fullPath);
            await this.apiClient.uploadFile(state.user, state.drop, relPath, fileStream, stat.size);
            this.emit("file-synced", { dropPath: state.dropPath, file: relPath, type: "upload" });
        } catch (err) {
            this.emit("sync-error", { dropPath: state.dropPath, file: relPath, error: err.message });
        }
    }

    stopSync(dropPath) {
        var state = this.activeSyncs.get(dropPath);
        if (state) {
            state.running = false;
            if (state.syncTimer) clearInterval(state.syncTimer);
            if (state.localTimer) clearInterval(state.localTimer);
            this._saveIndex(state);
            this.activeSyncs.delete(dropPath);
            console.log("[SyncManager] Stopped sync for " + dropPath);
        }
    }

    stopAllSync() {
        var self = this;
        this.activeSyncs.forEach(function(state, dropPath) {
            self.stopSync(dropPath);
        });
        console.log("[SyncManager] Stopped all syncs");
    }
}

module.exports = SyncManager;