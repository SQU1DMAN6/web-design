const path = require("path");
const os = require("os");
const fs = require("fs");

const SETTINGS_DIR = path.join(os.homedir(), ".inker");
const SETTINGS_FILE = path.join(SETTINGS_DIR, "settings.json");

/**
 * Settings manager - stores all data in a simple JSON file.
 * No native modules required (no SQLite, no node-gyp).
 */
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
                const raw = fs.readFileSync(SETTINGS_FILE, "utf8");
                this._data = JSON.parse(raw);
            }
        } catch (err) {
            console.error("Settings load error:", err.message);
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
            console.error("Settings save error:", err.message);
        }
    }

    async getAuth() {
        return this._data.auth || null;
    }

    async saveAuth(email, username, sessionId) {
        this._data.auth = { email, username, session_id: sessionId };
        this._save();
        return { email, username, sessionId };
    }

    async clearAuth() {
        this._data.auth = null;
        this._save();
    }

    async addMount(user, repo, mountPoint, autoMount = false) {
        const repoPath = `${user}/${repo}`;
        const idx = this._data.mounts.findIndex(m => m.repo_path === repoPath);
        const entry = { repo_path: repoPath, user, repo, mount_point: mountPoint, auto_mount: autoMount };
        if (idx >= 0) {
            this._data.mounts[idx] = entry;
        } else {
            this._data.mounts.push(entry);
        }
        this._save();
        return entry;
    }

    async getMounts() {
        return this._data.mounts || [];
    }

    async getMount(repoPath) {
        return this._data.mounts.find(m => m.repo_path === repoPath) || null;
    }

    async removeMount(repoPath) {
        this._data.mounts = this._data.mounts.filter(m => m.repo_path !== repoPath);
        this._save();
    }

    async setAutoMount(repoPath, autoMount) {
        const mount = this._data.mounts.find(m => m.repo_path === repoPath);
        if (mount) {
            mount.auto_mount = autoMount;
            this._save();
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