/**
 * Ephemeral LRU Cache
 * 
 * Invisible to the user. Stores hydrated file chunks temporarily.
 * Automatically evicts least-recently-used entries when capacity is reached.
 * Cleared on application restart.
 */

const fs = require("fs");
const path = require("path");
const os = require("os");
const crypto = require("crypto");

class CacheEntry {
    constructor(buffer, metadata) {
        this.buffer = buffer;
        this.metadata = metadata || {};
        this.lastAccess = Date.now();
        this.created = Date.now();
    }

    touch() {
        this.lastAccess = Date.now();
    }

    age() {
        return Date.now() - this.created;
    }
}

class LRUCache {
    constructor(options) {
        options = options || {};
        this.maxSize = options.maxSize || 512 * 1024 * 1024; // 512MB default
        this.maxAge = options.maxAge || 30 * 60 * 1000;     // 30 min default
        this.cacheDir = options.cacheDir || path.join(os.tmpdir(), ".inker-cache");

        this._ensureDir();

        // In-memory map: key -> CacheEntry
        this._store = new Map();
        // Usage order tracking (for LRU eviction)
        this._accessOrder = [];
        this._currentSize = 0;

        console.log("[Cache] Initialized (maxSize=" + this._formatSize(this.maxSize) + ", maxAge=" + (this.maxAge / 1000) + "s, dir=" + this.cacheDir + ")");
    }

    _ensureDir() {
        try {
            if (!fs.existsSync(this.cacheDir)) {
                fs.mkdirSync(this.cacheDir, { recursive: true });
            }
        } catch (e) {
            console.log("[Cache] Failed to create cache dir: " + e.message);
        }
    }

    _key(user, drop, filePath, offset, length) {
        var hash = crypto.createHash("md5").update(user + "/" + drop + "/" + filePath).digest("hex");
        return hash + ":" + offset + ":" + length;
    }

    _formatSize(bytes) {
        if (!bytes || bytes === 0) return "0 B";
        var units = ["B", "KB", "MB", "GB"];
        var i = 0;
        var size = bytes;
        while (size >= 1024 && i < units.length - 1) {
            size /= 1024;
            i++;
        }
        return Math.round(size * 10) / 10 + " " + units[i];
    }

    _touch(key) {
        var idx = this._accessOrder.indexOf(key);
        if (idx !== -1) {
            this._accessOrder.splice(idx, 1);
        }
        this._accessOrder.push(key);
    }

    _evict() {
        while (this._currentSize > this.maxSize && this._accessOrder.length > 0) {
            var oldestKey = this._accessOrder.shift();
            var entry = this._store.get(oldestKey);
            if (entry) {
                this._currentSize -= entry.buffer.length;
                this._store.delete(oldestKey);
                console.log("[Cache] Evicted " + oldestKey + " (" + this._formatSize(entry.buffer.length) + ")");
            }
        }
    }

    _purgeExpired() {
        var now = Date.now();
        var toDelete = [];
        this._store.forEach(function (entry, key) {
            if (now - entry.created > this.maxAge) {
                toDelete.push(key);
            }
        }, this);

        for (var i = 0; i < toDelete.length; i++) {
            var key = toDelete[i];
            var entry = this._store.get(key);
            if (entry) {
                this._currentSize -= entry.buffer.length;
                this._store.delete(key);
                var idx = this._accessOrder.indexOf(key);
                if (idx !== -1) this._accessOrder.splice(idx, 1);
            }
        }
        if (toDelete.length > 0) {
            console.log("[Cache] Purged " + toDelete.length + " expired entries");
        }
    }

    /**
     * Get a cached chunk. Returns buffer if found, null otherwise.
     */
    get(user, drop, filePath, offset, length) {
        var key = this._key(user, drop, filePath, offset, length);
        var entry = this._store.get(key);
        if (!entry) return null;

        // Check expiration
        if (Date.now() - entry.created > this.maxAge) {
            this._store.delete(key);
            this._currentSize -= entry.buffer.length;
            var idx = this._accessOrder.indexOf(key);
            if (idx !== -1) this._accessOrder.splice(idx, 1);
            return null;
        }

        entry.touch();
        this._touch(key);
        return entry.buffer;
    }

    /**
     * Store a chunk in the cache.
     */
    set(user, drop, filePath, offset, length, buffer, metadata) {
        var key = this._key(user, drop, filePath, offset, length);
        var entry = new CacheEntry(buffer, metadata);

        // If key already exists, remove old entry first
        var old = this._store.get(key);
        if (old) {
            this._currentSize -= old.buffer.length;
            var idx = this._accessOrder.indexOf(key);
            if (idx !== -1) this._accessOrder.splice(idx, 1);
        }

        this._store.set(key, entry);
        this._currentSize += buffer.length;
        this._touch(key);

        // Evict if over capacity
        this._evict();

        // Periodically purge expired entries (throttled)
        if (Math.random() < 0.1) {
            this._purgeExpired();
        }
    }

    /**
     * Check if a chunk is cached.
     */
    has(user, drop, filePath, offset, length) {
        var key = this._key(user, drop, filePath, offset, length);
        return this._store.has(key);
    }

    /**
     * Invalidate all cached chunks for a given file.
     */
    invalidateFile(user, drop, filePath) {
        var hash = crypto.createHash("md5").update(user + "/" + drop + "/" + filePath).digest("hex");
        var prefix = hash + ":";
        var toDelete = [];

        this._store.forEach(function (entry, key) {
            if (key.startsWith(prefix)) {
                toDelete.push(key);
            }
        });

        for (var i = 0; i < toDelete.length; i++) {
            var key = toDelete[i];
            var entry = this._store.get(key);
            if (entry) {
                this._currentSize -= entry.buffer.length;
                this._store.delete(key);
                var idx = this._accessOrder.indexOf(key);
                if (idx !== -1) this._accessOrder.splice(idx, 1);
            }
        }
        console.log("[Cache] Invalidated " + toDelete.length + " entries for " + user + "/" + drop + "/" + filePath);
    }

    /** Clear entire cache */
    clear() {
        this._store.clear();
        this._accessOrder = [];
        this._currentSize = 0;
        console.log("[Cache] Cleared");
    }

    /** Get cache statistics */
    stats() {
        return {
            entries: this._store.size,
            currentSize: this._currentSize,
            currentSizeFormatted: this._formatSize(this._currentSize),
            maxSize: this.maxSize,
            usagePercent: Math.round((this._currentSize / this.maxSize) * 100)
        };
    }
}

module.exports = LRUCache;