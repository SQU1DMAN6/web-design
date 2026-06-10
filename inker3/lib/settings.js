const path = require("path");
const os = require("os");
const fs = require("fs");

const SETTINGS_DIR = path.join(os.homedir(), ".inker");
const SETTINGS_FILE = path.join(SETTINGS_DIR, "settings.json");

class Settings {
    constructor() {
        this._data = { auth: null, mounts: [], settings: {} };
        this._loaded = false;
        this._loadSync();
    }

    _loadSync() {
        try {
            if (!fs.existsSync(SETTINGS_DIR)) {
                fs.mkdirSync(SETTINGS_DIR, { recursive: true });
            }
            if (fs.existsSync(SETTINGS_FILE)) {
                var raw = fs.readFileSync(SETTINGS_FILE, "utf8");
                this._data = JSON.parse(raw);
            }
        } catch (err) {
            console.error("[Settings] Load error: " + err.message);
            this._data = { auth: null, mounts: [], settings: {} };
        }
        this._loaded = true;
    }

    _save() {
        try {
            if (!fs.existsSync(SETTINGS_DIR)) {
                fs.mkdirSync(SETTINGS_DIR, { recursive: true });
            }
            fs.writeFileSync(SETTINGS_FILE, JSON.stringify(this._data, null, 2), "utf8");
        } catch (err) {
            console.error("[Settings] Save error: " + err.message);
        }
    }

    async getAuth() {
        return this._data.auth || null;
    }

    async saveAuth(email, username, sessionId) {
        this._data.auth = { email: email, username: username, session_id: sessionId };
        this._save();
        console.log("[Settings] Auth saved for " + username);
        return { email: email, username: username, sessionId: sessionId };
    }

    async clearAuth() {
        this._data.auth = null;
        this._save();
        console.log("[Settings] Auth cleared");
    }

    async addMount(user, drop, mountPoint, autoMount) {
        var dropPath = user + "/" + drop;
        var idx = -1;
        for (var i = 0; i < this._data.mounts.length; i++) {
            if (this._data.mounts[i].drop_path === dropPath) {
                idx = i;
                break;
            }
        }
        var entry = { drop_path: dropPath, user: user, drop: drop, mount_point: mountPoint, auto_mount: autoMount || false };
        if (idx >= 0) {
            this._data.mounts[idx] = entry;
        } else {
            this._data.mounts.push(entry);
        }
        this._save();
        console.log("[Settings] Mount added: " + dropPath);
        return entry;
    }

    async getMounts() {
        return this._data.mounts || [];
    }

    async getMount(dropPath) {
        for (var i = 0; i < this._data.mounts.length; i++) {
            if (this._data.mounts[i].drop_path === dropPath) {
                return this._data.mounts[i];
            }
        }
        return null;
    }

    async removeMount(dropPath) {
        this._data.mounts = this._data.mounts.filter(function (m) {
            return m.drop_path !== dropPath;
        });
        this._save();
        console.log("[Settings] Mount removed: " + dropPath);
    }

    async setAutoMount(dropPath, autoMount) {
        for (var i = 0; i < this._data.mounts.length; i++) {
            if (this._data.mounts[i].drop_path === dropPath) {
                this._data.mounts[i].auto_mount = autoMount;
                this._save();
                console.log("[Settings] Auto-mount " + (autoMount ? "enabled" : "disabled") + " for " + dropPath);
                break;
            }
        }
    }

    async getSetting(key) {
        return this._data.settings[key] || null;
    }

    async setSetting(key, value) {
        this._data.settings[key] = value;
        this._save();
        return value;
    }

    close() {
        this._save();
    }
}

module.exports = new Settings();