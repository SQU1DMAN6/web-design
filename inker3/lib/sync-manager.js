const fs = require("fs");
const path = require("path");
const { EventEmitter } = require("events");

class SyncManager extends EventEmitter {
    constructor(apiClient) {
        super();
        this.apiClient = apiClient;
        this.activeSyncs = new Map();
        this.remotePollInterval = 10000;
    }

    _shouldSkip(relPath) {
        if (!relPath) return true;
        var parts = relPath.split("/");
        for (var pi = 0; pi < parts.length; pi++) {
            if (parts[pi].startsWith(".")) return true;
        }
        if (relPath.endsWith(".lnk")) return true;
        return false;
    }

    _createUrlShortcut(mountPoint, relPath, user, drop) {
        var encodedPath = relPath.split("/").map(function(p) { return encodeURIComponent(p); }).join("/");
        var inkerUrl = "inker://" + user + "/" + drop + "/" + encodedPath;
        var shortcutPath = path.join(mountPoint, relPath + ".url");
        var parentDir = path.dirname(shortcutPath);
        fs.mkdirSync(parentDir, { recursive: true });
        if (!fs.existsSync(shortcutPath)) {
            fs.writeFileSync(shortcutPath, "[InternetShortcut]\r\nURL=" + inkerUrl + "\r\n");
        }
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
            remoteTimer: null,
            watcher: null,
            lastRemoteCheck: 0,
            knownFiles: new Map(),
            debounceTimers: new Map()
        };

        this._loadIndex(state);

        state.remoteTimer = setInterval((function(self, s) {
            self._syncRemote(s);
        }).bind(null, this, state), this.remotePollInterval);

        try {
            if (fs.existsSync(mountPoint)) {
                var self = this;
                var s = state;
                state.watcher = fs.watch(mountPoint, { recursive: true }, function(eventType, filename) {
                    self._onLocalChange(s, eventType, filename);
                });
                console.log("[SyncManager] fs.watch started for " + mountPoint);
            }
        } catch (err) {
            console.log("[SyncManager] fs.watch failed: " + err.message);
        }

        setTimeout((function(self, s) {
            self._syncRemote(s);
        }).bind(null, this, state), 1000);

        this.activeSyncs.set(dropPath, state);
        console.log("[SyncManager] Sync started for " + dropPath + " at " + mountPoint + " (remotePoll=" + this.remotePollInterval + "ms)");
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

    _onLocalChange(state, eventType, filename) {
        if (!state.running || !filename) return;
        var relPath = filename.replace(/\\/g, "/");

        // Skip hidden/metadata files but NOT .url files (we need to detect .url deletions)
        if (this._shouldSkip(relPath) && !relPath.endsWith(".url")) return;

        if (state.debounceTimers.has(relPath)) {
            clearTimeout(state.debounceTimers.get(relPath));
        }

        var timer = setTimeout((function(self, s, rp) {
            return function() {
                self._processLocalChange(s, rp);
                s.debounceTimers.delete(rp);
            };
        }).bind(null, this, state, relPath)(), 500);

        state.debounceTimers.set(relPath, timer);
    }

    _processLocalChange(state, relPath) {
        var fullPath = path.join(state.mountPoint, relPath);
        var known = state.knownFiles.get(relPath);
        var isUrlFile = relPath.endsWith(".url");

        try {
            if (fs.existsSync(fullPath)) {
                var stat = fs.statSync(fullPath);

                // If it's a .url shortcut or directory, skip upload
                if (isUrlFile || stat.isDirectory()) return;
                if (!stat.isFile()) return;

                var localMtime = stat.mtime.toISOString();
                var localSize = stat.size;

                if (!known) {
                    if (localSize === 0) {
                        // Empty new file — just track it without uploading or converting
                        // It will be uploaded when content is written later
                        state.knownFiles.set(relPath, {
                            mtime: localMtime,
                            size: 0,
                            kind: "file",
                            synced: false
                        });
                        this._saveIndex(state);
                        return;
                    }
                    // New file with content — upload, then convert to .url on success
                    var self = this;
                    var s = state;
                    var rp = relPath;
                    var mp = state.mountPoint;
                    var u = state.user;
                    var d = state.drop;
                    this._uploadLocalFile(state, relPath, function() {
                        // Upload succeeded — replace original with .url shortcut
                        var realPath = path.join(mp, rp);
                        try {
                            if (fs.existsSync(realPath) && fs.statSync(realPath).isFile() && !rp.endsWith(".url")) {
                                fs.unlinkSync(realPath);
                            }
                        } catch (e) {}
                        self._createUrlShortcut(mp, rp, u, d);
                        s.knownFiles.set(rp, {
                            mtime: new Date().toISOString(),
                            size: 0,
                            kind: "file",
                            synced: true
                        });
                        self._saveIndex(s);
                    });
                } else if (localMtime !== known.mtime || localSize !== known.size) {
                    // File changed — upload new content, then keep as .url
                    var self = this;
                    var s = state;
                    var rp = relPath;
                    var mp = state.mountPoint;
                    var u = state.user;
                    var d = state.drop;
                    this._uploadLocalFile(state, relPath, function() {
                        // Upload succeeded — replace with .url shortcut (in case it's a real file)
                        var realPath = path.join(mp, rp);
                        try {
                            if (fs.existsSync(realPath) && fs.statSync(realPath).isFile() && !rp.endsWith(".url")) {
                                fs.unlinkSync(realPath);
                            }
                        } catch (e) {}
                        self._createUrlShortcut(mp, rp, u, d);
                        s.knownFiles.set(rp, {
                            mtime: new Date().toISOString(),
                            size: 0,
                            kind: "file",
                            synced: true
                        });
                        self._saveIndex(s);
                    });
                }
            } else {
                // File deleted locally
                if (known) {
                    // A known file was deleted — delete from remote
                    state.knownFiles.delete(relPath);
                    this._deleteRemoteFile(state, relPath);
                    this.emit("file-synced", { dropPath: state.dropPath, file: relPath, type: "delete-local" });
                    this._saveIndex(state);
                } else if (isUrlFile) {
                    // A .url file was deleted — delete corresponding remote file
                    var remotePath = relPath.slice(0, -4);
                    var remoteKnown = state.knownFiles.get(remotePath);
                    if (remoteKnown) {
                        state.knownFiles.delete(remotePath);
                        this._deleteRemoteFile(state, remotePath);
                        this.emit("file-synced", { dropPath: state.dropPath, file: remotePath, type: "delete-local-via-url" });
                        this._saveIndex(state);
                    }
                }
            }
        } catch (err) {
            this.emit("sync-error", { dropPath: state.dropPath, file: relPath, error: err.message });
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
                if (this._shouldSkip(relPath)) continue;

                var isDir = entry.kind === "directory" || entry.type === "dir" || relPath.endsWith("/");
                var known = state.knownFiles.get(relPath);
                var remoteMtime = entry.modified || entry.mtime || "";
                var remoteSize = entry.size || 0;

                if (!known) {
                    // New remote file — create .url shortcut + directory if needed
                    state.knownFiles.set(relPath, {
                        mtime: remoteMtime,
                        size: remoteSize,
                        kind: isDir ? "directory" : "file",
                        synced: false
                    });
                    if (isDir) {
                        var localDir = path.join(state.mountPoint, relPath);
                        fs.mkdirSync(localDir, { recursive: true });
                    } else {
                        this._createUrlShortcut(state.mountPoint, relPath, state.user, state.drop);
                    }
                    changed = true;
                    this.emit("file-synced", { dropPath: state.dropPath, file: relPath, type: "remote-add" });
                } else if (!isDir && remoteMtime !== known.mtime) {
                    // Remote file changed — update index
                    known.mtime = remoteMtime;
                    known.size = remoteSize;
                    known.synced = false;
                    changed = true;
                    this.emit("file-synced", { dropPath: state.dropPath, file: relPath, type: "remote-change" });
                }
            }

            // Detect remote deletions — delete local .url files too
            var remotePaths = {};
            for (var i = 0; i < entries.length; i++) {
                var ep = entries[i].path || entries[i].name;
                if (ep) remotePaths[ep] = true;
            }
            var toDelete = [];
            state.knownFiles.forEach(function(info, rp) {
                if (!remotePaths[rp] && info.kind !== "directory") {
                    toDelete.push(rp);
                }
            });
            for (var di = 0; di < toDelete.length; di++) {
                var rp = toDelete[di];
                state.knownFiles.delete(rp);
                // Delete local .url shortcut
                var urlPath = path.join(state.mountPoint, rp + ".url");
                try {
                    if (fs.existsSync(urlPath)) {
                        fs.unlinkSync(urlPath);
                    }
                } catch (e) {}
                changed = true;
                this.emit("file-synced", { dropPath: state.dropPath, file: rp, type: "remote-delete" });
            }

            if (changed) {
                this._saveIndex(state);
            }
        } catch (err) {
            this.emit("sync-error", { dropPath: state.dropPath, file: "(remote poll)", error: err.message });
        }
    }

    _uploadLocalFile(state, relPath, callback) {
        try {
            var fullPath = path.join(state.mountPoint, relPath);
            if (!fs.existsSync(fullPath)) {
                if (callback) callback();
                return;
            }
            var stat = fs.statSync(fullPath);
            if (!stat.isFile() || stat.size === 0) {
                if (callback) callback();
                return;
            }

            var fileStream = fs.createReadStream(fullPath);
            var self = this;
            this.apiClient.uploadFile(state.user, state.drop, relPath, fileStream, stat.size)
            .then(function(result) {
                self.emit("file-synced", { dropPath: state.dropPath, file: relPath, type: "upload" });
                if (callback) callback();
            })
            .catch(function(err) {
                self.emit("sync-error", { dropPath: state.dropPath, file: relPath, error: err.message });
                if (callback) callback();
            });
        } catch (err) {
            this.emit("sync-error", { dropPath: state.dropPath, file: relPath, error: err.message });
            if (callback) callback();
        }
    }

    async _deleteRemoteFile(state, relPath) {
        try {
            await this.apiClient.deleteFile(state.user, state.drop, relPath);
        } catch (err) {
            // File may already be deleted on remote
        }
    }

    stopSync(dropPath) {
        var state = this.activeSyncs.get(dropPath);
        if (state) {
            state.running = false;
            if (state.remoteTimer) clearInterval(state.remoteTimer);
            if (state.watcher) {
                try { state.watcher.close(); } catch (e) {}
            }
            state.debounceTimers.forEach(function(t) { clearTimeout(t); });
            state.debounceTimers.clear();
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

    getSyncStatus(dropPath) {
        var state = this.activeSyncs.get(dropPath);
        if (!state) return null;
        return {
            dropPath: state.dropPath,
            running: state.running,
            knownFiles: state.knownFiles.size
        };
    }
}

module.exports = SyncManager;