/**
 * Virtual Filesystem Abstraction Layer
 * 
 * Maintains the virtual directory tree for mounted drops.
 * Provides a unified interface that both WinFsp and future FUSE
 * implementations can call into.
 * 
 * Key design: There is NO local state. The virtual tree is always
 * lazily constructed from InkDrop metadata. File contents are
 * streamed on demand.
 */

const path = require("path");
const { EventEmitter } = require("events");

class VirtualFS extends EventEmitter {
    constructor(hydrator) {
        super();
        this.hydrator = hydrator;

        // Active mounts: mountPoint -> { user, drop, entries[], mountedAt }
        this._mounts = new Map();

        // Reverse lookup: dropKey -> mountPoint
        this._dropToMount = new Map();

        console.log("[VirtualFS] Initialized");
    }

    _dropKey(user, drop) {
        return user + "/" + drop;
    }

    // ====== Mount Management ======

    /**
     * Mount a drop at a virtual mount point.
     * mountPoint is a logical path like "/user/drop" — WinFsp maps it to Q:\
     */
    async mount(user, drop) {
        var key = this._dropKey(user, drop);

        if (this._dropToMount.has(key)) {
            throw new Error(key + " is already mounted");
        }

        // Fetch the directory listing to verify the drop exists
        var entries = await this.hydrator.listDirectory(user, drop);

        var mountPoint = "/" + key; // Virtual path

        this._mounts.set(mountPoint, {
            user: user,
            drop: drop,
            entries: entries,
            mountedAt: Date.now()
        });

        this._dropToMount.set(key, mountPoint);

        console.log("[VirtualFS] Mounted " + key + " at " + mountPoint + " (" + entries.length + " entries)");

        // Start background prefetching
        this.hydrator.prefetchDirectory(user, drop).catch(function () {});

        this.emit("mounted", { user: user, drop: drop, mountPoint: mountPoint });
        return mountPoint;
    }

    /**
     * Unmount a drop.
     */
    async unmount(user, drop) {
        var key = this._dropKey(user, drop);
        var mountPoint = this._dropToMount.get(key);

        if (!mountPoint) {
            throw new Error(key + " is not mounted");
        }

        this._mounts.delete(mountPoint);
        this._dropToMount.delete(key);

        console.log("[VirtualFS] Unmounted " + key);
        this.emit("unmounted", { user: user, drop: drop });
    }

    /**
     * Get all active mounts.
     */
    getMounts() {
        var result = [];
        this._mounts.forEach(function (state, mountPoint) {
            result.push({
                mountPoint: mountPoint,
                user: state.user,
                drop: state.drop,
                entryCount: state.entries ? state.entries.length : 0
            });
        });
        return result;
    }

    /**
     * Check if a drop is mounted.
     */
    isMounted(user, drop) {
        return this._dropToMount.has(this._dropKey(user, drop));
    }

    /**
     * Refresh the entry listing for a mounted drop.
     */
    async refresh(user, drop) {
        var key = this._dropKey(user, drop);
        var mountPoint = this._dropToMount.get(key);
        if (!mountPoint) return;

        var state = this._mounts.get(mountPoint);
        if (!state) return;

        state.entries = await this.hydrator.refreshListing(user, drop);
        console.log("[VirtualFS] Refreshed " + key + ": " + state.entries.length + " entries");
    }

    // ====== File Operations ======

    /**
     * Get file/directory metadata for a virtual path.
     * Returns null if not found.
     */
    async stat(mountPoint, virtualPath) {
        var state = this._mounts.get(mountPoint);
        if (!state) return null;

        // Root directory
        if (virtualPath === "/" || virtualPath === "") {
            return {
                name: state.user + "/" + state.drop,
                size: 0,
                modified: state.mountedAt || 0,
                isDirectory: true,
                exists: true
            };
        }

        var cleanPath = virtualPath.replace(/^\/+/, "");

        // Search the cached entry list
        var entries = state.entries;
        if (!entries) return null;

        for (var i = 0; i < entries.length; i++) {
            var e = entries[i];
            if (e.path === cleanPath) {
                return {
                    name: cleanPath.split("/").pop(),
                    size: e.size || 0,
                    modified: e.modified || 0,
                    isDirectory: e.type === "dir",
                    exists: true,
                    hash: e.hash || null
                };
            }
        }

        // If not found in listing, try the stat API
        if (cleanPath.indexOf("/") !== -1 || !this._isInListing(entries, cleanPath)) {
            var stat = await this.hydrator.getFileInfo(state.user, state.drop, cleanPath);
            if (stat) {
                return {
                    name: cleanPath.split("/").pop(),
                    size: stat.size || 0,
                    modified: stat.modified || 0,
                    isDirectory: stat.isDirectory || false,
                    exists: true,
                    hash: stat.hash || null
                };
            }
        }

        return { exists: false };
    }

    _isInListing(entries, path) {
        for (var i = 0; i < entries.length; i++) {
            if (entries[i].path === path) return true;
        }
        return false;
    }

    /**
     * List the contents of a virtual directory.
     * Returns array of { name, size, modified, isDirectory }.
     */
    async readdir(mountPoint, virtualPath) {
        var state = this._mounts.get(mountPoint);
        if (!state) return [];

        var cleanParent = virtualPath.replace(/^\/+/, "");
        if (cleanParent === "") cleanParent = ""; // root

        var entries = state.entries || [];
        var children = [];

        var seen = {};

        for (var i = 0; i < entries.length; i++) {
            var e = entries[i];
            var relPath = e.path;

            // Check if this entry is a direct child of the requested directory
            var parentDir = this._parentPath(relPath);

            if (parentDir === cleanParent || (cleanParent === "" && parentDir === "")) {
                var name = this._baseName(relPath);
                if (name && !seen[name]) {
                    seen[name] = true;
                    children.push({
                        name: name,
                        size: e.type === "dir" ? 0 : (e.size || 0),
                        modified: e.modified || 0,
                        isDirectory: e.type === "dir"
                    });
                }
            }
        }

        // Sort: directories first, then files, alphabetically
        children.sort(function (a, b) {
            if (a.isDirectory && !b.isDirectory) return -1;
            if (!a.isDirectory && b.isDirectory) return 1;
            return a.name.localeCompare(b.name);
        });

        return children;
    }

    /**
     * Read a chunk of a file.
     */
    async read(mountPoint, virtualPath, offset, length) {
        var state = this._mounts.get(mountPoint);
        if (!state) throw new Error("Not mounted");

        var cleanPath = virtualPath.replace(/^\/+/, "");
        return await this.hydrator.readFileChunk(state.user, state.drop, cleanPath, offset, length);
    }

    /**
     * Write data to a file (create or overwrite).
     */
    async write(mountPoint, virtualPath, data) {
        var state = this._mounts.get(mountPoint);
        if (!state) throw new Error("Not mounted");

        var cleanPath = virtualPath.replace(/^\/+/, "");
        return await this.hydrator.writeFile(state.user, state.drop, cleanPath, data);
    }

    /**
     * Delete a file or empty directory.
     */
    async delete(mountPoint, virtualPath) {
        var state = this._mounts.get(mountPoint);
        if (!state) throw new Error("Not mounted");

        var cleanPath = virtualPath.replace(/^\/+/, "");
        return await this.hydrator.deleteFile(state.user, state.drop, cleanPath);
    }

    /**
     * Create a directory.
     */
    async mkdir(mountPoint, virtualPath) {
        var state = this._mounts.get(mountPoint);
        if (!state) throw new Error("Not mounted");

        var cleanPath = virtualPath.replace(/^\/+/, "");
        return await this.hydrator.createDirectory(state.user, state.drop, cleanPath);
    }

    /**
     * Rename a file or directory.
     */
    async rename(mountPoint, oldVirtualPath, newVirtualPath) {
        var state = this._mounts.get(mountPoint);
        if (!state) throw new Error("Not mounted");

        var cleanOld = oldVirtualPath.replace(/^\/+/, "");
        var cleanNew = newVirtualPath.replace(/^\/+/, "");
        return await this.hydrator.renameFile(state.user, state.drop, cleanOld, cleanNew);
    }

    // ====== Path Utilities ======

    _parentPath(relPath) {
        var idx = relPath.lastIndexOf("/");
        if (idx === -1) return "";
        return relPath.substring(0, idx);
    }

    _baseName(relPath) {
        var parts = relPath.split("/");
        return parts[parts.length - 1];
    }

    // ====== Lifecycle ======

    /**
     * Unmount all drops.
     */
    unmountAll() {
        var self = this;
        this._mounts.forEach(function (state, mountPoint) {
            self.emit("unmounted", { user: state.user, drop: state.drop });
        });
        this._mounts.clear();
        this._dropToMount.clear();
        this.hydrator.shutdown();
        console.log("[VirtualFS] All mounts cleared");
    }

    /**
     * Get stats about all mounts.
     */
    getStats() {
        var mounts = this._mounts.size;
        var entries = 0;
        this._mounts.forEach(function (state) {
            if (state.entries) entries += state.entries.length;
        });
        return {
            activeMounts: mounts,
            totalEntries: entries,
            cacheStats: this.hydrator.getCacheStats()
        };
    }
}

module.exports = VirtualFS;