const fs = require("fs");
const path = require("path");
const { EventEmitter } = require("events");

/**
 * MountManager (v3 - No-FUSE)
 *
 * Each "mount" is just a real local directory under (typically) %USERPROFILE%/FtR/<user>/<repo>.
 * The directory is created on mount, and SyncManager (chokidar) handles bidirectional
 * sync between local and remote. This is the same model OneDrive / Google Drive for Desktop
 * use on Windows, and it works without any kernel-level driver.
 */
class MountManager extends EventEmitter {
    constructor(apiClient) {
        super();
        this.apiClient = apiClient;
        this.activeMounts = new Map(); // repoPath -> { user, repo, mountPoint }
    }

    /**
     * Mount a repository. Ensures the local directory exists, registers it,
     * pre-populates with the current remote file list (downloading files into
     * the folder), and emits 'mounted' for SyncManager to attach to.
     */
    async mount(user, repo, mountPoint) {
        const repoPath = `${user}/${repo}`;

        if (this.activeMounts.has(repoPath)) {
            throw new Error(`${repoPath} is already mounted`);
        }

        // Make sure the local folder exists.
        fs.mkdirSync(mountPoint, { recursive: true });

        // Register the mount immediately so subsequent calls are consistent.
        this.activeMounts.set(repoPath, { user, repo, mountPoint });

        // Pre-populate the local folder from the remote in the background.
        // We do not block the mount on this - the user can start working
        // locally while the initial sync happens.
        this._initialPull(user, repo, mountPoint).catch((err) => {
            console.error(`Initial pull failed for ${repoPath}: ${err.message}`);
        });

        this.emit("mounted", { repoPath, mountPoint });
        return { repoPath, mountPoint };
    }

    /**
     * Download the remote file list and write any files that are missing locally.
     * Existing local files are left alone (local changes win on the first sync cycle).
     */
    async _initialPull(user, repo, mountPoint) {
        let entries = [];
        try {
            entries = await this.apiClient.getFileList(user, repo);
        } catch (err) {
            console.error(`getFileList failed for ${user}/${repo}: ${err.message}`);
            return;
        }

        for (const entry of entries) {
            const relPath = entry.path || entry.name;
            if (!relPath) continue;

            // entry.kind: 'file' | 'directory' (be permissive)
            const isDir = entry.kind === "directory" || entry.type === "dir" || relPath.endsWith("/");
            const localPath = path.join(mountPoint, relPath);

            if (isDir) {
                fs.mkdirSync(localPath, { recursive: true });
                continue;
            }

            if (fs.existsSync(localPath)) continue;

            try {
                const stream = await this.apiClient.downloadFile(user, repo, relPath);
                await new Promise((resolve, reject) => {
                    const out = fs.createWriteStream(localPath);
                    stream.on("error", reject);
                    out.on("error", reject);
                    out.on("finish", resolve);
                    stream.pipe(out);
                });
            } catch (err) {
                console.error(`Initial download failed for ${relPath}: ${err.message}`);
            }
        }
    }

    async unmount(repoPath) {
        if (!this.activeMounts.has(repoPath)) {
            throw new Error(`${repoPath} is not mounted`);
        }

        const { mountPoint } = this.activeMounts.get(repoPath);
        this.activeMounts.delete(repoPath);
        this.emit("unmounted", { repoPath, mountPoint });
        return { repoPath, mountPoint };
    }

    getActiveMounts() {
        return Array.from(this.activeMounts.entries()).map(([repoPath, state]) => ({
            repoPath,
            mountPoint: state.mountPoint
        }));
    }

    isMounted(repoPath) {
        return this.activeMounts.has(repoPath);
    }

    closeAll() {
        this.activeMounts.clear();
    }
}

module.exports = MountManager;
