/**
 * Streaming File Hydration Engine
 * 
 * The bridge between virtual file requests and the InkDrop API.
 * Handles:
 *   - On-demand file hydration (partial reads with Range headers)
 *   - Directory listing fetching with caching
 *   - File metadata lookups
 *   - Write-through uploads
 *   - Background prefetching for active directories
 */

const LRUCache = require("./cache");

class Hydrator {
    constructor(apiClient) {
        this.apiClient = apiClient;
        this.cache = new LRUCache({
            maxSize: 512 * 1024 * 1024,  // 512MB
            maxAge: 30 * 60 * 1000        // 30 minutes
        });

        // Directory listing cache: dropPath -> { entries, timestamp }
        this._dirCache = new Map();
        this._dirCacheTTL = 30 * 1000; // 30 seconds

        // Prefetch queue
        this._prefetchQueue = [];
        this._prefetchActive = false;
        this._prefetchChunkSize = 65536; // 64KB

        console.log("[Hydrator] Initialized");
    }

    _dropKey(user, drop) {
        return user + "/" + drop;
    }

    // ====== Directory Listing ======

    /**
     * Get directory listing for a drop. Returns cached version if fresh.
     */
    async listDirectory(user, drop) {
        var key = this._dropKey(user, drop);
        var cached = this._dirCache.get(key);

        if (cached && (Date.now() - cached.timestamp) < this._dirCacheTTL) {
            return cached.entries;
        }

        console.log("[Hydrator] Fetching listing for " + key);
        var entries = await this.apiClient.getFileList(user, drop);

        this._dirCache.set(key, {
            entries: entries,
            timestamp: Date.now()
        });

        return entries;
    }

    /**
     * Force refresh the directory listing.
     */
    async refreshListing(user, drop) {
        var key = this._dropKey(user, drop);
        this._dirCache.delete(key);
        return await this.listDirectory(user, drop);
    }

    /**
     * Invalidate directory listing cache for a drop.
     */
    invalidateListing(user, drop) {
        var key = this._dropKey(user, drop);
        this._dirCache.delete(key);
    }

    // ====== File Metadata ======

    /**
     * Get metadata for a single file. Checks directory cache first for performance.
     */
    async getFileInfo(user, drop, filePath) {
        // Try directory cache first
        var entries = await this.listDirectory(user, drop);
        for (var i = 0; i < entries.length; i++) {
            var e = entries[i];
            if (e.path === filePath && e.type !== "dir") {
                return {
                    path: e.path,
                    name: e.path.split("/").pop(),
                    size: e.size || 0,
                    modified: e.modified || 0,
                    hash: e.hash || null,
                    isDirectory: false
                };
            }
        }

        // Fallback to stat API
        var stat = await this.apiClient.statFile(user, drop, filePath);
        if (stat) {
            return {
                path: stat.path,
                name: stat.name || filePath.split("/").pop(),
                size: stat.size || 0,
                modified: stat.modified || 0,
                hash: stat.hash || null,
                isDirectory: stat.type === "dir"
            };
        }

        return null;
    }

    async getDirectoryInfo(user, drop, dirPath) {
        var entries = await this.listDirectory(user, drop);
        for (var i = 0; i < entries.length; i++) {
            var e = entries[i];
            if (e.path === dirPath && e.type === "dir") {
                return {
                    path: e.path,
                    name: e.path.split("/").pop(),
                    size: 0,
                    modified: e.modified || 0,
                    isDirectory: true
                };
            }
        }
        return null;
    }

    // ====== File Reading (Hydration) ======

    /**
     * Read a chunk of a file. Returns a Buffer.
     * Uses cache if available, otherwise fetches from API.
     */
    async readFileChunk(user, drop, filePath, offset, length) {
        // Check cache first
        var cached = this.cache.get(user, drop, filePath, offset, length);
        if (cached) {
            return cached;
        }

        // Fetch from remote with Range header
        console.log("[Hydrator] Fetching chunk " + user + "/" + drop + "/" + filePath + " offset=" + offset + " length=" + length);
        var responseStream = await this.apiClient.streamFile(user, drop, filePath, offset, length);
        var buffer = await this._streamToBuffer(responseStream);

        // Cache the result
        this.cache.set(user, drop, filePath, offset, length, buffer);

        return buffer;
    }

    /**
     * Read an entire file into a Buffer.
     */
    async readFullFile(user, drop, filePath) {
        var info = await this.getFileInfo(user, drop, filePath);
        var size = info ? info.size : 0;

        // For small files, fetch entirely
        if (size > 0 && size <= 10 * 1024 * 1024) {
            return await this.readFileChunk(user, drop, filePath, 0, size);
        }

        // For larger files, use the full stream download
        console.log("[Hydrator] Full download " + user + "/" + drop + "/" + filePath);
        return await this.apiClient.downloadFile(user, drop, filePath);
    }

    // ====== File Writing (Write-Through) ======

    /**
     * Write a file. Invalidates cache for that file.
     */
    async writeFile(user, drop, filePath, data) {
        console.log("[Hydrator] Writing " + user + "/" + drop + "/" + filePath + " (" + data.length + " bytes)");

        // Upload to server
        var result = await this.apiClient.uploadFile(user, drop, filePath, data, data.length);

        // Invalidate cache entries for this file
        this.cache.invalidateFile(user, drop, filePath);

        // Invalidate directory listing
        this.invalidateListing(user, drop);

        return result;
    }

    /**
     * Delete a file. Invalidates cache.
     */
    async deleteFile(user, drop, filePath) {
        console.log("[Hydrator] Deleting " + user + "/" + drop + "/" + filePath);
        var result = await this.apiClient.deleteFile(user, drop, filePath);
        this.cache.invalidateFile(user, drop, filePath);
        this.invalidateListing(user, drop);
        return result;
    }

    /**
     * Create a directory.
     */
    async createDirectory(user, drop, dirPath) {
        console.log("[Hydrator] Creating directory " + user + "/" + drop + "/" + dirPath);
        var result = await this.apiClient.createDirectory(user, drop, dirPath);
        this.invalidateListing(user, drop);
        return result;
    }

    /**
     * Rename a file/directory.
     */
    async renameFile(user, drop, oldPath, newPath) {
        console.log("[Hydrator] Renaming " + oldPath + " -> " + newPath);
        var result = await this.apiClient.renameFile(user, drop, oldPath, newPath);
        this.cache.invalidateFile(user, drop, oldPath);
        this.invalidateListing(user, drop);
        return result;
    }

    // ====== Prefetching ======

    /**
     * Prefetch the first chunk of each file in a drop's listing.
     * Used when a directory becomes "active" (user browses it).
     */
    async prefetchDirectory(user, drop) {
        var entries = await this.listDirectory(user, drop);
        var files = entries.filter(function (e) { return e.type !== "dir"; });

        console.log("[Hydrator] Prefetching " + Math.min(files.length, 10) + " files from " + user + "/" + drop);

        // Only prefetch first 10 files to avoid overloading
        var count = Math.min(files.length, 10);
        for (var i = 0; i < count; i++) {
            var file = files[i];
            this._enqueuePrefetch(user, drop, file.path, file.size);
        }
    }

    _enqueuePrefetch(user, drop, filePath, fileSize) {
        this._prefetchQueue.push({
            user: user,
            drop: drop,
            path: filePath,
            size: fileSize || 0
        });
        this._processPrefetchQueue();
    }

    async _processPrefetchQueue() {
        if (this._prefetchActive) return;
        this._prefetchActive = true;

        while (this._prefetchQueue.length > 0) {
            var item = this._prefetchQueue.shift();
            try {
                // Only prefetch if not already cached
                if (!this.cache.has(item.user, item.drop, item.path, 0, this._prefetchChunkSize)) {
                    var chunkSize = Math.min(this._prefetchChunkSize, item.size || this._prefetchChunkSize);
                    if (chunkSize > 0) {
                        var buffer = await this.apiClient.downloadFile(item.user, item.drop, item.path);
                        var firstChunk = buffer.slice(0, chunkSize);
                        this.cache.set(item.user, item.drop, item.path, 0, firstChunk.length, firstChunk, {
                            prefetched: true
                        });
                    }
                }
            } catch (e) {
                // Silent failure for prefetch
            }
        }

        this._prefetchActive = false;
    }

    // ====== Utility ======

    _streamToBuffer(stream) {
        return new Promise(function (resolve, reject) {
            var chunks = [];
            stream.on("data", function (chunk) { chunks.push(chunk); });
            stream.on("end", function () { resolve(Buffer.concat(chunks)); });
            stream.on("error", reject);
        });
    }

    /** Clear all caches */
    clearCaches() {
        this.cache.clear();
        this._dirCache.clear();
        console.log("[Hydrator] All caches cleared");
    }

    /** Get cache statistics */
    getCacheStats() {
        return this.cache.stats();
    }

    /** Close/shutdown */
    shutdown() {
        this.clearCaches();
    }
}

module.exports = Hydrator;