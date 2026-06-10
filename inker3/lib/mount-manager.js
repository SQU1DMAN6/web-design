const fs = require("fs");
const path = require("path");
const { EventEmitter } = require("events");

class MountManager extends EventEmitter {
    constructor(apiClient) {
        super();
        this.apiClient = apiClient;
        this.activeMounts = new Map();
    }

    async mount(user, drop, mountPoint) {
        var dropPath = user + "/" + drop;

        if (this.activeMounts.has(dropPath)) {
            throw new Error(dropPath + " is already mounted");
        }

        fs.mkdirSync(mountPoint, { recursive: true });
        this.activeMounts.set(dropPath, { user: user, drop: drop, mountPoint: mountPoint });

        console.log("[MountManager] Mounted " + dropPath + " at " + mountPoint);

        this._initialPull(user, drop, mountPoint).catch(function (err) {
            console.error("[MountManager] Initial pull failed for " + dropPath + ": " + err.message);
        });

        this.emit("mounted", { dropPath: dropPath, mountPoint: mountPoint });
        return { dropPath: dropPath, mountPoint: mountPoint };
    }

    async _initialPull(user, drop, mountPoint) {
        var entries = [];
        try {
            entries = await this.apiClient.getFileList(user, drop);
        } catch (err) {
            console.error("[MountManager] getFileList failed for " + user + "/" + drop + ": " + err.message);
            return;
        }

        console.log("[MountManager] Initial pull: " + entries.length + " entries for " + user + "/" + drop);

        for (var i = 0; i < entries.length; i++) {
            var entry = entries[i];
            var relPath = entry.path || entry.name;
            if (!relPath) continue;

            var isDir = entry.kind === "directory" || entry.type === "dir" || relPath.endsWith("/");
            var localPath = path.join(mountPoint, relPath);

            if (isDir) {
                fs.mkdirSync(localPath, { recursive: true });
                continue;
            }

            if (fs.existsSync(localPath)) continue;

            try {
                var stream = await this.apiClient.downloadFile(user, drop, relPath);
                await new Promise(function (resolve, reject) {
                    var out = fs.createWriteStream(localPath);
                    stream.on("error", reject);
                    out.on("error", reject);
                    out.on("finish", resolve);
                    stream.pipe(out);
                });
                console.log("[MountManager] Downloaded " + relPath);
            } catch (err) {
                console.error("[MountManager] Download failed for " + relPath + ": " + err.message);
            }
        }
    }

    async unmount(dropPath) {
        if (!this.activeMounts.has(dropPath)) {
            throw new Error(dropPath + " is not mounted");
        }

        var mountPoint = this.activeMounts.get(dropPath).mountPoint;
        this.activeMounts.delete(dropPath);
        console.log("[MountManager] Unmounted " + dropPath);
        this.emit("unmounted", { dropPath: dropPath, mountPoint: mountPoint });
        return { dropPath: dropPath, mountPoint: mountPoint };
    }

    getActiveMounts() {
        var result = [];
        this.activeMounts.forEach(function (state, dropPath) {
            result.push({ dropPath: dropPath, mountPoint: state.mountPoint });
        });
        return result;
    }

    isMounted(dropPath) {
        return this.activeMounts.has(dropPath);
    }

    closeAll() {
        this.activeMounts.clear();
    }
}

module.exports = MountManager;