const https = require("https");
const http = require("http");
const { URL } = require("url");

// Allow self-signed certificates (common for self-hosted InkDrop servers).
process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";

class InkDropClient {
    constructor() {
        this.baseURL = "https://inkdrop.quanthai.net";
        this.sessionId = null;
        this.email = null;
        this.username = null;
    }

    setSession(email, username, sessionId) {
        this.email = email;
        this.username = username;
        this.sessionId = sessionId;
    }

    _makeRequest(method, path, body = null, isJson = false) {
        return new Promise((resolve, reject) => {
            const url = new URL(path.startsWith("http") ? path : this.baseURL + path);
            const isHttps = url.protocol === "https:";
            const client = isHttps ? https : http;

            const options = {
                method,
                headers: {
                    "User-Agent": "FTR-Inker-3",
                    "X-FTR-CLIENT": "FtR-Inker-3"
                }
            };

            if (isJson) {
                options.headers["Content-Type"] = "application/json";
            } else if (body && method !== "GET") {
                options.headers["Content-Type"] = "application/x-www-form-urlencoded";
            }

            if (this.sessionId) {
                options.headers["Cookie"] = `PHPSESSID=${this.sessionId}`;
            }

            if (body && method !== "GET") {
                if (isJson) {
                    options.headers["Content-Length"] = Buffer.byteLength(body);
                } else {
                    options.headers["Content-Length"] = Buffer.byteLength(body);
                }
            }

            const req = client.request(url, options, (res) => {
                let data = "";
                res.on("data", (chunk) => { data += chunk; });
                res.on("end", () => {
                    if (res.statusCode >= 400) {
                        reject(new Error(`HTTP ${res.statusCode}: ${data}`));
                    } else {
                        try {
                            if (isJson && data) resolve(JSON.parse(data));
                            else resolve(data);
                        } catch (e) {
                            resolve(data);
                        }
                    }
                });
            });

            req.on("error", reject);
            if (body && method !== "GET") req.write(body);
            req.end();
        });
    }

    async login(email, password) {
        const body = `email=${encodeURIComponent(email)}&password=${encodeURIComponent(password)}`;
        const response = await this._makeRequest("POST", "/login.php", body);
        
        if (response.includes("Error logging in")) {
            throw new Error("Invalid credentials");
        }

        const userMatch = response.match(/Logged in as <b>([^<]+)<\/b>/);
        const username = userMatch ? userMatch[1] : email;

        this.email = email;
        this.username = username;
        return { email, username };
    }

    async searchRepositories(query) {
        const path = `/index.php?search=${encodeURIComponent(query)}&api=1`;
        try {
            const result = await this._makeRequest("GET", path, null, true);
            return result.repositories || [];
        } catch (e) {
            console.error("Search failed:", e.message);
            return [];
        }
    }

    async listRepositories() {
        const path = `/index.php?list=1&api=1`;
        try {
            const result = await this._makeRequest("GET", path, null, true);
            return result.repositories || [];
        } catch (e) {
            console.error("List repositories failed:", e.message);
            return [];
        }
    }

    async getFileList(user, repo) {
        const path = `/api/fs/${encodeURIComponent(user)}/${encodeURIComponent(repo)}?list=1`;
        try {
            const result = await this._makeRequest("GET", path, null, true);
            if (!result.success) throw new Error(result.error || "Failed to list files");
            return result.entries || [];
        } catch (e) {
            console.error(`List files failed for ${user}/${repo}:`, e.message);
            return [];
        }
    }

    async downloadFile(user, repo, filePath) {
        const encodedPath = filePath.split("/").map(p => encodeURIComponent(p)).join("/");
        const path = `/api/fs/${encodeURIComponent(user)}/${encodeURIComponent(repo)}/${encodedPath}`;
        return new Promise((resolve, reject) => {
            const url = new URL(this.baseURL + path);
            const isHttps = url.protocol === "https:";
            const client = isHttps ? https : http;

            const options = {
                headers: {
                    "User-Agent": "FTR-Inker-3",
                    "X-FTR-CLIENT": "FtR-Inker-3"
                }
            };

            if (this.sessionId) {
                options.headers["Cookie"] = `PHPSESSID=${this.sessionId}`;
            }

            const req = client.request(url, options, (res) => {
                if (res.statusCode !== 200) {
                    reject(new Error(`HTTP ${res.statusCode}`));
                } else {
                    resolve(res);
                }
            });

            req.on("error", reject);
            req.end();
        });
    }

    async uploadFile(user, repo, filePath, fileStream, size) {
        const encodedPath = filePath.split("/").map(p => encodeURIComponent(p)).join("/");
        const path = `/api/fs/${encodeURIComponent(user)}/${encodeURIComponent(repo)}/${encodedPath}`;
        return new Promise((resolve, reject) => {
            const url = new URL(this.baseURL + path);
            const isHttps = url.protocol === "https:";
            const client = isHttps ? https : http;

            const options = {
                method: "PUT",
                headers: {
                    "User-Agent": "FTR-Inker-3",
                    "X-FTR-CLIENT": "FtR-Inker-3",
                    "Content-Length": size
                }
            };

            if (this.sessionId) {
                options.headers["Cookie"] = `PHPSESSID=${this.sessionId}`;
            }

            const req = client.request(url, options, (res) => {
                let data = "";
                res.on("data", (chunk) => { data += chunk; });
                res.on("end", () => {
                    if (res.statusCode >= 400) {
                        reject(new Error(`HTTP ${res.statusCode}: ${data}`));
                    } else {
                        try {
                            resolve(JSON.parse(data));
                        } catch (e) {
                            resolve({ success: true });
                        }
                    }
                });
            });

            req.on("error", reject);
            fileStream.pipe(req);
        });
    }

    async deleteFile(user, repo, filePath) {
        const encodedPath = filePath.split("/").map(p => encodeURIComponent(p)).join("/");
        const path = `/api/fs/${encodeURIComponent(user)}/${encodeURIComponent(repo)}/${encodedPath}`;
        const result = await this._makeRequest("DELETE", path, null, true);
        if (!result.success) throw new Error(result.error || "Failed to delete file");
        return result;
    }

    async createDirectory(user, repo, dirPath) {
        const encodedPath = dirPath.split("/").map(p => encodeURIComponent(p)).join("/");
        const path = `/api/fs/${encodeURIComponent(user)}/${encodeURIComponent(repo)}/${encodedPath}?op=mkdir`;
        const result = await this._makeRequest("POST", path, JSON.stringify({}), true);
        if (!result.success) throw new Error(result.error || "Failed to create directory");
        return result;
    }

    async renameFile(user, repo, oldPath, newPath) {
        const encodedPath = oldPath.split("/").map(p => encodeURIComponent(p)).join("/");
        const path = `/api/fs/${encodeURIComponent(user)}/${encodeURIComponent(repo)}/${encodedPath}?op=rename`;
        const body = JSON.stringify({ newPath });
        const result = await this._makeRequest("POST", path, body, true);
        if (!result.success) throw new Error(result.error || "Failed to rename file");
        return result;
    }
}

module.exports = new InkDropClient();
