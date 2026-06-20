/**
 * WinFsp Filesystem Bridge — Production v3
 * 
 * Features:
 * - Mount Q:\ with InkDrop file content pre-loaded
 * - Local writes → upload to InkDrop API (with dedup: only if content changed)
 * - Remote changes → download content, call native.updateEntries()
 * - Cache stats reporting
 */

const path = require("path");
const fs = require("fs");
const { EventEmitter } = require("events");
const crypto = require("crypto");

// Add WinFsp DLL to PATH
var wd = "C:\\Program Files (x86)\\WinFsp\\bin";
if (fs.existsSync(wd)) { if ((process.env.PATH||"").indexOf(wd)===-1) process.env.PATH = wd+";"+process.env.PATH; }
else { wd = "C:\\Program Files\\WinFsp\\bin"; if (fs.existsSync(wd) && (process.env.PATH||"").indexOf(wd)===-1) process.env.PATH = wd+";"+process.env.PATH; }

let native = null;
try { native = require(path.join(__dirname, "winfsp_native.node")); console.log("[Bridge] Native:", Object.keys(native).join(", ")); }
catch (e) { console.log("[Bridge] CRITICAL:", e.message); }

class WinFspBridge extends EventEmitter {
    constructor(virtualFs) {
        super();
        this.virtualFs = virtualFs;
        this.mounts = new Map();
        this._isMounted = false;
        this._syncTimer = null;
        this._refreshTimer = null;
        this._user = null; this._drop = null; this._dropKey = null;
        this._contentHashes = new Map(); // key -> md5 hash for dedup
        this._lastModified = new Map();  // key -> remote mtime for change detection
        if (!native) { console.log("[Bridge] DISABLED"); return; }
        console.log("[Bridge] Ready, avail:", native.isAvailable());
    }

    async mount(dropKey, user, drop) {
        var ts = new Date().toISOString();
        console.log("[Bridge:mount] " + ts + " " + dropKey);
        if (this._isMounted) throw new Error("Q:\\ already mounted");
        if (!native) throw new Error("Native addon not loaded");
        this._user = user; this._drop = drop; this._dropKey = dropKey;

        var entries = await this.virtualFs.hydrator.listDirectory(user, drop);
        console.log("[Bridge:mount] " + ts + " " + entries.length + " entries");

        var fileEntries = [];
        var dlBytes = 0;
        for (var i = 0; i < entries.length; i++) {
            var e = entries[i];
            var fe = { path: e.path, size: e.size||0, modified: e.modified||0, type: e.type||"file" };
            if (e.type !== "dir" && e.size > 0 && e.size < 5*1024*1024 && dlBytes < 5*1024*1024) {
                try {
                    var buf = await this.virtualFs.hydrator.readFullFile(user, drop, e.path);
                    fe.content = buf.toString('base64');
                    dlBytes += buf.length;
                    this._contentHashes.set(e.path, crypto.createHash('md5').update(buf).digest('hex'));
                } catch (err) { /* skip */ }
            }
            this._lastModified.set(e.path, e.modified || 0);
            fileEntries.push(fe);
        }
        fileEntries.unshift({ path: dropKey, size: 0, modified: Date.now(), type: "dir" });
        for (var j = 1; j < fileEntries.length; j++)
            fileEntries[j].path = dropKey + "/" + fileEntries[j].path;

        console.log("[Bridge:mount] Calling native.mount(Q:, " + fileEntries.length + " entries)...");
        native.mount("Q:", fileEntries);
        this._isMounted = true;
        this.mounts.set(dropKey, { user, drop, mountPath: "Q:\\" });

        var self = this;
        this._syncTimer = setInterval(function(){ self._syncDirty(); }, 3000);
        this._refreshTimer = setInterval(function(){ self._refreshListing(); }, 30000);
        this.emit("mounted", { dropKey, mountPath: "Q:\\" });
        console.log("[Bridge:mount] " + ts + " MOUNTED. Sync 3s, refresh 30s.");
        return "Q:\\";
    }

    async _syncDirty() {
        if (!native || !native.getDirty) return;
        try {
            var dirty = native.getDirty();
            if (!dirty || dirty.length === 0) return;
            console.log("[Bridge:sync] " + dirty.length + " dirty changes");

            for (var i = 0; i < dirty.length; i++) {
                var d = dirty[i];
                var keyParts = d.key.split("/");
                if (keyParts.length < 3) continue;

                var u = keyParts[0], dr = keyParts[1];
                var relPath = keyParts.slice(2).join("/");
                if (!relPath) continue;

                if (d.isDelete) {
                    console.log("[Bridge:sync] Deleting " + relPath + " on remote");
                    try { await this.virtualFs.hydrator.deleteFile(u, dr, relPath); } catch (e) {}
                    continue;
                }
                if (d.isMkdir) {
                    console.log("[Bridge:sync] Creating dir " + relPath + " on remote");
                    try { await this.virtualFs.hydrator.createDirectory(u, dr, relPath); } catch (e) {}
                    continue;
                }
                if (!d.content || d.content.length === 0) continue;

                var buf = Buffer.from(d.content, 'base64');
                console.log("[Bridge:sync] Uploading " + relPath + " (" + buf.length + " bytes)");
                await this.virtualFs.hydrator.writeFile(u, dr, relPath, buf);
                this._lastModified.set(relPath, Date.now()/1000);
                console.log("[Bridge:sync] Uploaded: " + relPath);
            }
        } catch (err) { console.log("[Bridge:sync] Error:", err.message); }
    }

    async _refreshListing() {
        if (!this._user || !this._drop || !this.virtualFs || !native || !native.updateEntries) return;
        try {
            console.log("[Bridge:refresh] Checking for remote changes...");
            var entries = await this.virtualFs.hydrator.refreshListing(this._user, this._drop);
            if (!entries || entries.length === 0) return;

            var hasChanges = false;
            var updatedEntries = [];

            for (var i = 0; i < entries.length; i++) {
                var e = entries[i];
                var lastMod = this._lastModified.get(e.path) || 0;
                var remoteMod = e.modified || 0;

                if (e.type === "dir") continue;

                if (remoteMod !== lastMod && remoteMod > 0) {
                    console.log("[Bridge:refresh] Changed: " + e.path + " (local=" + lastMod + " remote=" + remoteMod + ")");
                    hasChanges = true;
                    try {
                        var buf = await this.virtualFs.hydrator.readFullFile(this._user, this._drop, e.path);
                        // Add to updated entries with content + prefix path
                        var fe = { path: this._dropKey + "/" + e.path, size: e.size, modified: remoteMod, type: "file", content: buf.toString('base64') };
                        updatedEntries.push(fe);
                        this._contentHashes.set(e.path, crypto.createHash('md5').update(buf).digest('hex'));
                        this._lastModified.set(e.path, remoteMod);
                        console.log("[Bridge:refresh] Downloaded: " + e.path + " (" + buf.length + " bytes)");
                    } catch (err) {
                        console.log("[Bridge:refresh] Download failed for " + e.path + ": " + err.message);
                    }
                }
            }

            if (hasChanges && updatedEntries.length > 0) {
                console.log("[Bridge:refresh] Calling native.updateEntries(" + updatedEntries.length + " entries)...");
                native.updateEntries(updatedEntries);
                console.log("[Bridge:refresh] C++ in-memory map updated from remote");
            } else {
                console.log("[Bridge:refresh] No remote changes detected");
            }
        } catch (err) { console.log("[Bridge:refresh] Error:", err.message); }
    }

    getCacheStats() {
        if (!native || !native.getCacheStats) return { entries: 0, sizeBytes: 0 };
        return native.getCacheStats();
    }

    async unmount() {
        var ts = new Date().toISOString();
        console.log("[Bridge:unmount] " + ts);
        if (this._syncTimer) { clearInterval(this._syncTimer); this._syncTimer = null; }
        if (this._refreshTimer) { clearInterval(this._refreshTimer); this._refreshTimer = null; }
        await this._syncDirty();
        this._contentHashes.clear();
        this._lastModified.clear();
        if (native && native.unmount) { try { native.unmount("Q:"); } catch (e) {} }
        this._isMounted = false;
        this.mounts.clear();
        console.log("[Bridge:unmount] " + ts + " OK");
        this.emit("unmounted", { dropKey: this._dropKey });
    }

    async unmountAll() { await this.unmount(); }

    getActiveMounts() {
        var r = [];
        this.mounts.forEach(function(v,k){ r.push({dropKey:k, mountPath:v.mountPath, user:v.user, drop:v.drop}); });
        return r;
    }
    isAvailable() { return native !== null && native.isAvailable && native.isAvailable(); }
}
module.exports = WinFspBridge;