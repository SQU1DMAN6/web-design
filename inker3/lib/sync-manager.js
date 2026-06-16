const fs = require("fs");
const path = require("path");
const os = require("os");
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

    _startWatcher(pathToWatch, label, callback) {
        try {
            if (!fs.existsSync(pathToWatch)) return null;
            var w = fs.watch(pathToWatch, { recursive: true }, function(eventType, filename) {
                try { callback(eventType, filename); } catch (e) {
                    if (e.message && e.message.includes("EPERM")) {
                        console.log("[SyncManager] watcher EPERM ignored for " + label + ": " + (filename || "?"));
                    } else {
                        console.log("[SyncManager] watcher callback error for " + label + ": " + e.message);
                    }
                }
            });
            w.on("error", function(err) {
                console.log("[SyncManager] watcher error event for " + label + " " + pathToWatch + ": " + err.message);
                try { w.close(); } catch (e) {}
            });
            console.log("[SyncManager] fs.watch started for " + label + " " + pathToWatch);
            return w;
        } catch (err) {
            console.log("[SyncManager] fs.watch failed for " + label + " " + pathToWatch + ": " + err.message);
            return null;
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
            cacheDir: path.join(os.homedir(), ".inker", "cache", user, drop),
            running: true,
            remoteTimer: null,
            watcher: null,
            cacheWatcher: null,
            lastRemoteCheck: 0,
            knownFiles: new Map(),
            debounceTimers: new Map(),
            uploading: new Set(),
            failedUploads: new Map()
        };

        this._loadIndex(state);

        state.remoteTimer = setInterval((function(self, s) {
            self._syncRemote(s);
        }).bind(null, this, state), this.remotePollInterval);

        var self = this;
        var s = state;
        state.watcher = this._startWatcher(mountPoint, "mount", function(eventType, filename) {
            self._onLocalChange(s, eventType, filename);
        });

        try { fs.mkdirSync(state.cacheDir, { recursive: true }); } catch (e) {}
        state.cacheWatcher = this._startWatcher(state.cacheDir, "cache", function(eventType, filename) {
            if (!s.running || !filename) return;
            var relPath = filename.replace(/\\/g, "/");
            if (relPath.startsWith(".")) return;
            if (s.debounceTimers.has("cache:" + relPath)) {
                clearTimeout(s.debounceTimers.get("cache:" + relPath));
            }
            var timer = setTimeout(function() {
                s.debounceTimers.delete("cache:" + relPath);
                self._processCacheChange(s, relPath);
            }, 500);
            s.debounceTimers.set("cache:" + relPath, timer);
        });

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

                if (isUrlFile || stat.isDirectory()) return;
                if (!stat.isFile()) return;

                var localMtime = stat.mtime.toISOString();
                var localSize = stat.size;

                if (!known) {
                    if (localSize === 0) {
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
                    if (state.uploading.has(relPath)) return;
                    var self = this;
                    var s = state;
                    var rp = relPath;
                    var mp = state.mountPoint;
                    var u = state.user;
                    var d = state.drop;
                    this._uploadLocalFile(state, relPath, function(err) {
                        self._finalizeUpload(s, rp, mp, u, d, err);
                    });
                } else if (localMtime !== known.mtime || localSize !== known.size) {
                    // File changed — upload new content, then keep as .url
                    if (state.uploading.has(relPath)) return;
                    var self = this;
                    var s = state;
                    var rp = relPath;
                    var mp = state.mountPoint;
                    var u = state.user;
                    var d = state.drop;
                    this._uploadLocalFile(state, relPath, function(err) {
                        self._finalizeUpload(s, rp, mp, u, d, err);
                    });
                }
            } else {
                if (known) {
                    // Check if .url shortcut still exists — if so, file was synced
                    // and the real file being removed is expected behavior
                    var urlShortcut = path.join(state.mountPoint, relPath + ".url");
                    var hasUrlShortcut = false;
                    try { hasUrlShortcut = fs.existsSync(urlShortcut); } catch (e) {}

                    if (hasUrlShortcut) {
                        // .url shortcut exists — file was uploaded and replaced with shortcut.
                        // The real file being removed is expected, do NOT delete from remote.
                        this.emit("file-synced", { dropPath: state.dropPath, file: relPath, type: "synced-real-removed" });
                    } else {
                        // No .url shortcut — true deletion, remove from remote
                        state.knownFiles.delete(relPath);
                        this._deleteRemoteFile(state, relPath);
                        this.emit("file-synced", { dropPath: state.dropPath, file: relPath, type: "delete-local" });
                        this._saveIndex(state);
                    }
                } else if (isUrlFile) {
                    // .url file deleted — check if real file is replacing it
                    var remotePath = relPath.slice(0, -4);
                    var remoteKnown = state.knownFiles.get(remotePath);
                    if (remoteKnown) {
                        var realPath = path.join(state.mountPoint, remotePath);
                        var realFileExists = false;
                        try { realFileExists = fs.existsSync(realPath) && fs.statSync(realPath).isFile(); } catch (e) {}
                        if (realFileExists) {
                            // Real file is replacing the .url — just clear known entry; upload will handle it
                            state.knownFiles.delete(remotePath);
                            this._saveIndex(state);
                            this.emit("file-synced", { dropPath: state.dropPath, file: remotePath, type: "replace-url-with-real" });
                        } else {
                            // True deletion — remove from remote
                            state.knownFiles.delete(remotePath);
                            this._deleteRemoteFile(state, remotePath);
                            this.emit("file-synced", { dropPath: state.dropPath, file: remotePath, type: "delete-local-via-url" });
                            this._saveIndex(state);
                        }
                    }
                }
            }
        } catch (err) {
            this.emit("sync-error", { dropPath: state.dropPath, file: relPath, error: err.message });
        }
    }

    _finalizeUpload(state, relPath, mountPoint, user, drop, err) {
        if (err) {
            console.log("[SyncManager] Upload failed (will retry): " + relPath + ": " + err);
            state.failedUploads.set(relPath, Date.now());
            this.emit("sync-error", { dropPath: state.dropPath, file: relPath, error: err });
            return;
        }
        state.failedUploads.delete(relPath);
        var realPath = path.join(mountPoint, relPath);
        try {
            if (fs.existsSync(realPath) && fs.statSync(realPath).isFile() && !relPath.endsWith(".url")) {
                fs.unlinkSync(realPath);
            }
        } catch (e) {}
        this._createUrlShortcut(mountPoint, relPath, user, drop);
        state.knownFiles.set(relPath, {
            mtime: new Date().toISOString(),
            size: 0,
            kind: "file",
            synced: true
        });
        this._saveIndex(state);
    }

    _processCacheChange(state, filename) {
        if (!state.running) return;
        var fullPath = path.join(state.cacheDir, filename);
        try {
            if (!fs.existsSync(fullPath)) return;
            var stat = fs.statSync(fullPath);
            if (!stat.isFile() || stat.size === 0) return;

            var relPath = filename;

            console.log("[SyncManager] Cache change detected, uploading: " + relPath);

            var fileStream = fs.createReadStream(fullPath);
            var self = this;
            var s = state;
            this.apiClient.uploadFile(state.user, state.drop, relPath, fileStream, stat.size)
            .then(function(result) {
                self.emit("file-synced", { dropPath: s.dropPath, file: relPath, type: "upload-cache" });
                console.log("[SyncManager] Cache upload complete: " + relPath);
                s.knownFiles.set(relPath, {
                    mtime: new Date().toISOString(),
                    size: stat.size,
                    kind: "file",
                    synced: true
                });
                self._saveIndex(s);
            })
            .catch(function(err) {
                self.emit("sync-error", { dropPath: s.dropPath, file: relPath, error: err.message });
            });
        } catch (err) {
            this.emit("sync-error", { dropPath: state.dropPath, file: filename, error: err.message });
        }
    }

    _downloadToCache(state, relPath) {
        var fileName = relPath.split("/").pop();
        var cachePath = path.join(state.cacheDir, fileName);
        var self = this;
        this.apiClient.downloadFile(state.user, state.drop, relPath).then(function(stream) {
            var out = fs.createWriteStream(cachePath);
            stream.pipe(out);
            out.on("finish", function() {
                console.log("[SyncManager] Updated cache for " + relPath);
            });
        }).catch(function(err) {
            console.log("[SyncManager] Cache update failed for " + relPath + ": " + err.message);
        });
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
                    // New remote file — create .url shortcut
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
                    // Remote file changed — update index, recreate .url, update cache
                    known.mtime = remoteMtime;
                    known.size = remoteSize;
                    known.synced = false;
                    changed = true;
                    try {
                        var urlPath = path.join(state.mountPoint, relPath + ".url");
                        if (fs.existsSync(urlPath)) {
                            fs.unlinkSync(urlPath);
                        }
                    } catch (e) {}
                    this._createUrlShortcut(state.mountPoint, relPath, state.user, state.drop);
                    this._downloadToCache(state, relPath);
                    this.emit("file-synced", { dropPath: state.dropPath, file: relPath, type: "remote-change" });
                }
            }

            // Detect remote deletions — delete local .url files
            var remotePaths = {};
            for (var i = 0; i < entries.length; i++) {
                var ep = entries[i].path || entries[i].name;
                if (ep) remotePaths[ep] = true;
            }
            var toDelete = [];
            state.knownFiles.forEach(function(info, rp) {
                if (!remotePaths[rp] && info.kind !== "directory" && !state.uploading.has(rp)) {
                    toDelete.push(rp);
                }
            });
            for (var di = 0; di < toDelete.length; di++) {
                var rp = toDelete[di];
                state.knownFiles.delete(rp);
                var urlPath = path.join(state.mountPoint, rp + ".url");
                try {
                    if (fs.existsSync(urlPath)) {
                        fs.unlinkSync(urlPath);
                    }
                } catch (e) {}
                changed = true;
                this.emit("file-synced", { dropPath: state.dropPath, file: rp, type: "remote-delete" });
            }

            // Retry failed uploads
            var self = this;
            state.failedUploads.forEach(function(timestamp, retryPath) {
                if (state.uploading.has(retryPath)) return;
                var retryFullPath = path.join(state.mountPoint, retryPath);
                if (!fs.existsSync(retryFullPath)) {
                    state.failedUploads.delete(retryPath);
                    return;
                }
                var retryStat;
                try { retryStat = fs.statSync(retryFullPath); } catch (e) { state.failedUploads.delete(retryPath); return; }
                if (!retryStat.isFile() || retryStat.size === 0) {
                    state.failedUploads.delete(retryPath);
                    return;
                }
                console.log("[SyncManager] Retrying upload: " + retryPath);
                self._uploadLocalFile(state, retryPath, function(err) {
                    self._finalizeUpload(state, retryPath, state.mountPoint, state.user, state.drop, err);
                });
            });

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
                if (callback) callback("File not found");
                return;
            }
            var stat = fs.statSync(fullPath);
            if (!stat.isFile() || stat.size === 0) {
                if (callback) callback("Not a valid file or empty");
                return;
            }

            state.uploading.add(relPath);
            var fileStream = fs.createReadStream(fullPath);
            var self = this;
            var s = state;
            this.apiClient.uploadFile(state.user, state.drop, relPath, fileStream, stat.size)
            .then(function(result) {
                s.uploading.delete(relPath);
                self.emit("file-synced", { dropPath: s.dropPath, file: relPath, type: "upload" });
                if (callback) callback(null);
            })
            .catch(function(err) {
                s.uploading.delete(relPath);
                self.emit("sync-error", { dropPath: s.dropPath, file: relPath, error: err.message });
                if (callback) callback(err.message);
            });
        } catch (err) {
            state.uploading.delete(relPath);
            this.emit("sync-error", { dropPath: state.dropPath, file: relPath, error: err.message });
            if (callback) callback(err.message);
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
            if (state.cacheWatcher) {
                try { state.cacheWatcher.close(); } catch (e) {}
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