const fs = require("fs");
const path = require("path");
const { EventEmitter } = require("events");
const chokidar = require("chokidar");

/**
 * SyncManager
 *
 * Watches each mounted local directory with chokidar and uploads changes
 * (add / change / unlink) to the remote repository. Also polls the remote
 * periodically to detect files that changed server-side.
 */
class SyncManager extends EventEmitter {
    constructor(apiClient) {
        super();
        this.apiClient = apiClient;
        this.activeWatchers = new Map();    // repoPath -> chokidar watcher
        this.fileSyncState = new Map();     // repoPath -> { syncTimer, user, repo, mountPoint }
        this.syncInterval = 5000;
    }

    startSync(repoPath, mountPoint) {
        if (this.activeWatchers.has(repoPath)) {
            return;
        }

        const [user, repo] = repoPath.split("/");

        const watcher = chokidar.watch(mountPoint, {
            ignored: /(^|[/\\])\..*|node_modules/,
            persistent: true,
            ignoreInitial: true,
            awaitWriteFinish: { stabilityThreshold: 500, pollInterval: 100 }
        });

        watcher
            .on("add", (filepath) => this._onFileChanged(repoPath, mountPoint, filepath, "upload"))
            .on("change", (filepath) => this._onFileChanged(repoPath, mountPoint, filepath, "update"))
            .on("unlink", (filepath) => this._onFileDeleted(repoPath, mountPoint, filepath))
            .on("addDir", (dirpath) => this._onDirAdded(repoPath, mountPoint, dirpath))
            .on("unlinkDir", (dirpath) => this._onDirDeleted(repoPath, mountPoint, dirpath))
            .on("error", (err) => this.emit("sync-error", { repoPath, error: err.message }))
            .on("ready", () => {
                this.emit("sync-ready", { repoPath });
            });

        this.activeWatchers.set(repoPath, watcher);
        this.fileSyncState.set(repoPath, { user, repo, mountPoint });

        this._startRemoteSync(repoPath);
    }

    stopSync(repoPath) {
        const watcher = this.activeWatchers.get(repoPath);
        if (watcher) {
            watcher.close().catch(() => {});
            this.activeWatchers.delete(repoPath);
        }

        const state = this.fileSyncState.get(repoPath);
        if (state && state.syncTimer) {
            clearInterval(state.syncTimer);
        }
        this.fileSyncState.delete(repoPath);
    }

    _relPath(mountPoint, filepath) {
        const rel = path.relative(mountPoint, filepath).replace(/\\/g, "/");
        if (rel.startsWith("..") || path.isAbsolute(rel)) return null;
        return rel;
    }

    async _onFileChanged(repoPath, mountPoint, filepath, type) {
        const relPath = this._relPath(mountPoint, filepath);
        if (!relPath) return;

        let stat;
        try {
            stat = fs.statSync(filepath);
        } catch (err) {
            return; // file vanished between event and stat
        }
        if (!stat.isFile()) return;

        const [user, repo] = repoPath.split("/");

        try {
            const fileStream = fs.createReadStream(filepath);
            await this.apiClient.uploadFile(user, repo, relPath, fileStream, stat.size);
            this.emit("file-synced", { repoPath, file: relPath, type });
        } catch (err) {
            this.emit("sync-error", { repoPath, file: relPath, error: err.message });
        }
    }

    async _onFileDeleted(repoPath, mountPoint, filepath) {
        const relPath = this._relPath(mountPoint, filepath);
        if (!relPath) return;

        const [user, repo] = repoPath.split("/");
        try {
            await this.apiClient.deleteFile(user, repo, relPath);
            this.emit("file-synced", { repoPath, file: relPath, type: "delete" });
        } catch (err) {
            this.emit("sync-error", { repoPath, file: relPath, error: err.message });
        }
    }

    async _onDirAdded(repoPath, mountPoint, dirpath) {
        const relPath = this._relPath(mountPoint, dirpath);
        if (!relPath) return;

        const [user, repo] = repoPath.split("/");
        try {
            await this.apiClient.createDirectory(user, repo, relPath);
            this.emit("file-synced", { repoPath, file: relPath, type: "mkdir" });
        } catch (err) {
            // mkdir on the server is best-effort; if it already exists that's fine.
            this.emit("sync-error", { repoPath, file: relPath, error: err.message });
        }
    }

    async _onDirDeleted(repoPath, mountPoint, dirpath) {
        const relPath = this._relPath(mountPoint, dirpath);
        if (!relPath) return;

        const [user, repo] = repoPath.split("/");
        try {
            await this.apiClient.deleteFile(user, repo, relPath);
            this.emit("file-synced", { repoPath, file: relPath, type: "rmdir" });
        } catch (err) {
            this.emit("sync-error", { repoPath, file: relPath, error: err.message });
        }
    }

    _startRemoteSync(repoPath) {
        const [user, repo] = repoPath.split("/");
        const syncTimer = setInterval(async () => {
            try {
                const files = await this.apiClient.getFileList(user, repo);
                this.emit("remote-sync-check", { repoPath, files });
            } catch (err) {
                this.emit("sync-error", { repoPath, error: err.message });
            }
        }, this.syncInterval);

        const state = this.fileSyncState.get(repoPath) || {};
        state.syncTimer = syncTimer;
        this.fileSyncState.set(repoPath, state);
    }

    stopAllSync() {
        this.activeWatchers.forEach((watcher) => {
            try {
                watcher.close();
            } catch (e) {
                console.error("Error stopping watcher:", e.message);
            }
        });
        this.fileSyncState.forEach((state) => {
            if (state.syncTimer) clearInterval(state.syncTimer);
        });
        this.activeWatchers.clear();
        this.fileSyncState.clear();
    }
}

module.exports = SyncManager;
