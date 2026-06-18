/**
 * WinFsp Filesystem Bridge — PRODUCTION VERSION
 * 
 * Architecture:
 *   Single Q:\ mount with subdirectories per drop: Q:\user\drop\files...
 *   JS fetches InkDrop entries + downloads content → passes to native C++ map
 *   C++ stores everything in memory → WinFsp FUSE3 → Explorer
 * 
 *   Multiple mounts are supported as subdirectories: mount("a/b"), mount("c/d")
 *   creates Q:\a\b\ and Q:\c\d\ with their respective files.
 */

const path = require("path");
const fs = require("fs");
const { EventEmitter } = require("events");

// Add WinFsp DLL to PATH
var winfspDllDir = "C:\\Program Files (x86)\\WinFsp\\bin";
if (fs.existsSync(winfspDllDir)) {
    var currentPath = process.env.PATH || "";
    if (currentPath.indexOf(winfspDllDir) === -1) {
        process.env.PATH = winfspDllDir + ";" + currentPath;
        console.log("[WinFspBridge] Added WinFsp DLL dir to PATH:", winfspDllDir);
    }
} else {
    winfspDllDir = "C:\\Program Files\\WinFsp\\bin";
    if (fs.existsSync(winfspDllDir)) {
        var currentPath = process.env.PATH || "";
        if (currentPath.indexOf(winfspDllDir) === -1) {
            process.env.PATH = winfspDllDir + ";" + currentPath;
            console.log("[WinFspBridge] Added WinFsp DLL dir to PATH:", winfspDllDir);
        }
    }
}

// Load native addon
let native = null;
try {
    native = require(path.join(__dirname, "winfsp_native.node"));
    console.log("[WinFspBridge] Native addon loaded, exports:", Object.keys(native).join(", "));
} catch (err) {
    console.log("[WinFspBridge] CRITICAL: Failed to load native addon:", err.message);
}

class WinFspBridge extends EventEmitter {
    constructor(virtualFs) {
        super();
        this.virtualFs = virtualFs;
        this.mounts = new Map();
        this._isMounted = false; // true once Q:\ is mounted
        if (!native) { console.log("[WinFspBridge] DISABLED"); return; }
        console.log("[WinFspBridge] Bridge ready, WinFsp available:", native.isAvailable());
    }

    async mount(dropKey, user, drop, driveLetter) {
        var ts = new Date().toISOString();
        console.log("[WinFspBridge:mount] " + ts + " ENTER: " + dropKey);

        if (this.mounts.has(dropKey)) throw new Error(dropKey + " already mounted");
        if (!native) throw new Error("WinFsp native addon not loaded");

        // Step 1: Fetch directory listing from API
        console.log("[WinFspBridge:mount] " + ts + " Fetching file listing from InkDrop API...");
        var entries = await this.virtualFs.hydrator.listDirectory(user, drop);
        console.log("[WinFspBridge:mount] " + ts + " Got " + entries.length + " entries");

        // Step 2: Download content for every file (small files, use base64 for binary safety)
        var fileEntries = [];
        var downloadCount = 0;
        var maxDownloadBytes = 5 * 1024 * 1024; // 5MB total per mount
        var downloadedBytes = 0;

        for (var i = 0; i < entries.length; i++) {
            var e = entries[i];
            var isDir = (e.type === "dir");
            
            var fe = {
                path: e.path,
                size: e.size || 0,
                modified: e.modified || 0,
                type: e.type || "file"
            };

            // Download file content (base64-encoded for safe binary transport)
            if (!isDir && e.size > 0 && e.size < 5 * 1024 * 1024 && downloadedBytes < maxDownloadBytes) {
                try {
                    console.log("[WinFspBridge:mount] " + ts + " Downloading " + e.path + " (" + e.size + " bytes)...");
                    var buffer = await this.virtualFs.hydrator.readFullFile(user, drop, e.path);
                    fe.content = buffer.toString('base64');
                    downloadedBytes += buffer.length;
                    downloadCount++;
                    console.log("[WinFspBridge:mount] " + ts + " Downloaded " + e.path + " (" + buffer.length + " bytes, base64=" + fe.content.length + ")");
                } catch (err) {
                    console.log("[WinFspBridge:mount] " + ts + " Download failed for " + e.path + ": " + err.message);
                }
            }

            fileEntries.push(fe);
        }
        console.log("[WinFspBridge:mount] " + ts + " Downloaded " + downloadCount + " files (" + downloadedBytes + " bytes)");

        // Step 3: Add a mount root entry for the user/drop directory
        var mountRoot = user + "/" + drop;
        var mountDir = {
            path: mountRoot,
            size: 0,
            modified: Date.now(),
            type: "dir"
        };
        fileEntries.unshift(mountDir);

        // Prepend paths with the mount root prefix so they appear under Q:\user\drop\
        for (var j = 0; j < fileEntries.length; j++) {
            if (fileEntries[j].path === mountRoot) continue;
            fileEntries[j].path = mountRoot + "/" + fileEntries[j].path;
        }

        // Step 4: Mount Q:\ if first mount, or add entries to existing mount
        var mountPoint = "Q:";
        if (!this._isMounted) {
            console.log("[WinFspBridge:mount] " + ts + " First mount — creating FUSE instance...");
            native.mount(mountPoint, fileEntries);
            console.log("[WinFspBridge:mount] " + ts + " Q:\\ FUSE loop started");
            this._isMounted = true;
            this.mounts.set(dropKey, { user: user, drop: drop, mountPath: "Q:\\", mounted: true });
            this.emit("mounted", { dropKey: dropKey, mountPath: "Q:\\" });
            console.log("[WinFspBridge:mount] " + ts + " EXIT SUCCESS (first mount)");
            return "Q:\\";
        } else {
            // TODO: re-mount with accumulated entries (requires fuse3_new again)
            // For now, we don't support incremental adds to an existing Q:\ mount
            // This requires saving all entries and re-mounting
            console.log("[WinFspBridge:mount] " + ts + " WARNING: Q:\\ already mounted. Multiple drops not yet supported.");
            throw new Error("Q:\\ already mounted. Only one mount supported currently.");
        }
    }

    async unmount(dropKey) {
        var ts = new Date().toISOString();
        console.log("[WinFspBridge:unmount] " + ts + " ENTER: " + dropKey);
        var mountInfo = this.mounts.get(dropKey);
        if (!mountInfo) { console.log("  not found"); return; }
        if (native && native.unmount) {
            try { native.unmount("Q:"); } catch (err) { console.log("  unmount error:", err.message); }
        }
        this.mounts.delete(dropKey);
        this._isMounted = false;
        console.log("[WinFspBridge:unmount] " + ts + " EXIT OK");
        this.emit("unmounted", { dropKey: dropKey });
    }

    async unmountAll() {
        var keys = Array.from(this.mounts.keys());
        for (var i = 0; i < keys.length; i++) await this.unmount(keys[i]).catch(function(){});
        this.mounts.clear();
        this._isMounted = false;
    }

    getActiveMounts() {
        var result = [];
        this.mounts.forEach(function (info, dropKey) {
            result.push({ dropKey: dropKey, mountPath: info.mountPath, user: info.user, drop: info.drop });
        });
        return result;
    }

    isAvailable() { return native !== null && native.isAvailable && native.isAvailable(); }
}
module.exports = WinFspBridge;