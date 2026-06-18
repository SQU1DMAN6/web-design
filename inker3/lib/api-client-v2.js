/**
 * InkDrop API Client v2
 * 
 * The single persistent communication layer between Inker and InkDrop.
 * Supports:
 *   - Full file streaming with HTTP Range headers (partial reads)
 *   - Metadata listing (?list=1)
 *   - Single-file stat (?stat=1)
 *   - Full file upload (PUT)
 *   - File/directory deletion (DELETE)
 *   - Directory creation (POST ?op=mkdir)
 *   - Rename (POST ?op=rename)
 *   - Authentication (login, session confirm, logout)
 *   - Drop search
 */

const https = require("https");
const http = require("http");
const { URL } = require("url");

process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";

class InkDropClientV2 {
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

    _timestamp() {
        return new Date().toISOString().replace("T", " ").slice(0, 19);
    }

    _buildHeaders(extra) {
        var headers = {
            "User-Agent": "FTR-Inker-3.2",
            "X-FTR-CLIENT": "FtR-Inker-3.2"
        };
        if (this.sessionId) {
            headers["Cookie"] = "PHPSESSID=" + this.sessionId;
        }
        if (extra) {
            for (var k in extra) {
                if (extra.hasOwnProperty(k)) {
                    headers[k] = extra[k];
                }
            }
        }
        return headers;
    }

    _makeRequest(method, path, body, isJson) {
        var self = this;
        return new Promise(function (resolve, reject) {
            var url = new URL(path.startsWith("http") ? path : self.baseURL + path);
            var isHttps = url.protocol === "https:";
            var client = isHttps ? https : http;
            var ts = self._timestamp();

            console.log("[APIv2] " + ts + " " + method + " " + path);

            var options = {
                method: method,
                headers: self._buildHeaders()
            };

            if (isJson) {
                options.headers["Content-Type"] = "application/json";
            } else if (body && method !== "GET") {
                options.headers["Content-Type"] = "application/x-www-form-urlencoded";
            }

            if (body && method !== "GET") {
                options.headers["Content-Length"] = Buffer.byteLength(body);
            }

            var req = client.request(url, options, function (res) {
                console.log("[APIv2] " + ts + " " + method + " " + path + " -> " + res.statusCode);

                var setCookie = res.headers["set-cookie"];
                if (setCookie) {
                    for (var ci = 0; ci < setCookie.length; ci++) {
                        var cookieStr = setCookie[ci];
                        var parts = cookieStr.split(";");
                        if (parts.length > 0) {
                            var nv = parts[0].split("=");
                            if (nv.length === 2 && nv[0] === "PHPSESSID") {
                                self.sessionId = nv[1];
                                console.log("[APIv2] " + ts + " Captured session ID");
                            }
                        }
                    }
                }

                var data = "";
                res.on("data", function (chunk) { data += chunk; });
                res.on("end", function () {
                    if (res.statusCode >= 400) {
                        console.log("[APIv2 ERROR] " + ts + " " + method + " " + path + " HTTP " + res.statusCode);
                        reject(new Error("HTTP " + res.statusCode + ": " + data.substring(0, 500)));
                    } else {
                        try {
                            if (isJson && data) {
                                resolve(JSON.parse(data));
                            } else {
                                resolve(data);
                            }
                        } catch (e) {
                            resolve(data);
                        }
                    }
                });
            });

            req.on("error", function (err) {
                console.log("[APIv2 ERROR] " + ts + " " + method + " " + path + " network: " + err.message);
                reject(err);
            });

            if (body && method !== "GET") {
                req.write(body);
            }
            req.end();
        });
    }

    /**
     * Stream a file (or part of a file) from remote.
     * If offset/length are provided, uses Range header for partial reads.
     * Returns a readable stream.
     */
    streamFile(user, drop, filePath, offset, length) {
        var self = this;
        var ts = this._timestamp();
        var encodedPath = filePath.split("/").map(function (p) { return encodeURIComponent(p); }).join("/");
        var path = "/api/fs/" + encodeURIComponent(user) + "/" + encodeURIComponent(drop) + "/" + encodedPath;

        return new Promise(function (resolve, reject) {
            var url = new URL(self.baseURL + path);
            var isHttps = url.protocol === "https:";
            var client = isHttps ? https : http;

            var headers = self._buildHeaders();
            if (offset !== undefined && length !== undefined) {
                var rangeEnd = offset + length - 1;
                headers["Range"] = "bytes=" + offset + "-" + rangeEnd;
                console.log("[APIv2] " + ts + " Partial read " + offset + "-" + (offset + length - 1) + " of " + filePath);
            } else {
                console.log("[APIv2] " + ts + " Full stream: " + filePath);
            }

            var options = {
                headers: headers
            };

            var req = client.request(url, options, function (res) {
                console.log("[APIv2] " + ts + " Stream response " + filePath + " -> " + res.statusCode);
                if (res.statusCode === 200 || res.statusCode === 206) {
                    resolve(res);
                } else {
                    var data = "";
                    res.on("data", function (chunk) { data += chunk; });
                    res.on("end", function () {
                        reject(new Error("HTTP " + res.statusCode + ": " + data.substring(0, 500)));
                    });
                }
            });

            req.on("error", function (err) {
                reject(err);
            });
            req.end();
        });
    }

    /**
     * Download an entire file as a buffer.
     */
    async downloadFile(user, drop, filePath) {
        var stream = await this.streamFile(user, drop, filePath);
        return new Promise(function (resolve, reject) {
            var chunks = [];
            stream.on("data", function (chunk) { chunks.push(chunk); });
            stream.on("end", function () { resolve(Buffer.concat(chunks)); });
            stream.on("error", reject);
        });
    }

    /**
     * Upload a file via PUT. Accepts a Buffer or a readable stream.
     */
    uploadFile(user, drop, filePath, data, size) {
        var self = this;
        var ts = this._timestamp();
        var encodedPath = filePath.split("/").map(function (p) { return encodeURIComponent(p); }).join("/");
        var path = "/api/fs/" + encodeURIComponent(user) + "/" + encodeURIComponent(drop) + "/" + encodedPath;

        return new Promise(function (resolve, reject) {
            var url = new URL(self.baseURL + path);
            var isHttps = url.protocol === "https:";
            var client = isHttps ? https : http;

            var headers = self._buildHeaders();
            headers["Content-Length"] = size || Buffer.byteLength(data);

            var options = {
                method: "PUT",
                headers: headers
            };

            var req = client.request(url, options, function (res) {
                var responseData = "";
                res.on("data", function (chunk) { responseData += chunk; });
                res.on("end", function () {
                    if (res.statusCode >= 400) {
                        reject(new Error("HTTP " + res.statusCode + ": " + responseData));
                    } else {
                        try {
                            resolve(JSON.parse(responseData));
                        } catch (e) {
                            resolve({ success: true });
                        }
                    }
                });
            });

            req.on("error", function (err) {
                console.log("[APIv2] " + ts + " Upload network error: " + err.message);
                reject(err);
            });

            if (Buffer.isBuffer(data)) {
                req.end(data);
            } else {
                data.pipe(req);
            }
        });
    }

    /** Delete a file or directory */
    async deleteFile(user, drop, filePath) {
        var encodedPath = filePath.split("/").map(function (p) { return encodeURIComponent(p); }).join("/");
        var path = "/api/fs/" + encodeURIComponent(user) + "/" + encodeURIComponent(drop) + "/" + encodedPath;
        var result = await this._makeRequest("DELETE", path, null, true);
        if (!result.success) throw new Error(result.error || "Failed to delete file");
        return result;
    }

    /** Get recursive file listing for a drop */
    async getFileList(user, drop) {
        var ts = this._timestamp();
        var path = "/api/fs/" + encodeURIComponent(user) + "/" + encodeURIComponent(drop) + "?list=1";
        console.log("[APIv2] " + ts + " Listing files for " + user + "/" + drop);
        try {
            var result = await this._makeRequest("GET", path, null, true);
            if (!result.success) throw new Error(result.error || "Failed to list files");
            return result.entries || [];
        } catch (e) {
            console.log("[APIv2] " + ts + " List failed for " + user + "/" + drop + ": " + e.message);
            return [];
        }
    }

    /** Stat a single file/directory in a drop */
    async statFile(user, drop, filePath) {
        var encodedPath = filePath.split("/").map(function (p) { return encodeURIComponent(p); }).join("/");
        var path = "/api/fs/" + encodeURIComponent(user) + "/" + encodeURIComponent(drop) + "/" + encodedPath + "?stat=1";
        try {
            var result = await this._makeRequest("GET", path, null, true);
            if (!result.success) throw new Error(result.error || "Failed to stat");
            return result.entry || null;
        } catch (e) {
            return null;
        }
    }

    /** Stat a drop root */
    async statDrop(user, drop) {
        var path = "/api/fs/" + encodeURIComponent(user) + "/" + encodeURIComponent(drop) + "?stat=1";
        try {
            var result = await this._makeRequest("GET", path, null, true);
            if (!result.success) throw new Error(result.error || "Failed to stat drop");
            return result;
        } catch (e) {
            return null;
        }
    }

    /** Create a directory */
    async createDirectory(user, drop, dirPath) {
        var encodedPath = dirPath.split("/").map(function (p) { return encodeURIComponent(p); }).join("/");
        var path = "/api/fs/" + encodeURIComponent(user) + "/" + encodeURIComponent(drop) + "/" + encodedPath + "?op=mkdir";
        var result = await this._makeRequest("POST", path, JSON.stringify({}), true);
        if (!result.success) throw new Error(result.error || "Failed to create directory");
        return result;
    }

    /** Rename a file or directory */
    async renameFile(user, drop, oldPath, newPath) {
        var encodedOld = oldPath.split("/").map(function (p) { return encodeURIComponent(p); }).join("/");
        var path = "/api/fs/" + encodeURIComponent(user) + "/" + encodeURIComponent(drop) + "/" + encodedOld + "?op=rename";
        var result = await this._makeRequest("POST", path, JSON.stringify({ newPath: newPath }), true);
        if (!result.success) throw new Error(result.error || "Failed to rename file");
        return result;
    }

    // ====== Auth methods ======

    async login(email, password) {
        var ts = this._timestamp();
        console.log("[APIv2] " + ts + " Login attempt for " + email);
        var body = "email=" + encodeURIComponent(email) + "&password=" + encodeURIComponent(password);
        this.email = email;
        var response = await this._makeRequest("POST", "/login.php", body);
        if (response.includes("Error logging in")) {
            this.sessionId = null;
            throw new Error("Invalid credentials");
        }
        var userMatch = response.match(/Logged in as <b>([^<]+)<\/b>/);
        var username = userMatch ? userMatch[1] : email;
        this.username = username;
        return { email: email, username: username };
    }

    async sessionConfirm() {
        var ts = this._timestamp();
        console.log("[APIv2] " + ts + " Session confirm...");
        try {
            var result = await this._makeRequest("GET", "/sessionconfirm", null, true);
            if (result && result.success === true) {
                if (result.username) this.username = result.username;
                return {
                    valid: true,
                    email: this.email,
                    username: this.username || result.username
                };
            }
            return { valid: false };
        } catch (e) {
            return { valid: false };
        }
    }

    async searchDrops(query) {
        var path = "/index.php?search=" + encodeURIComponent(query) + "&api=1";
        try {
            var result = await this._makeRequest("GET", path, null, true);
            var drops = [];
            if (result.matches && Array.isArray(result.matches)) {
                for (var i = 0; i < result.matches.length; i++) {
                    var m = result.matches[i];
                    drops.push({ user: m.user || "", name: m.repo || m.name || "", description: m.description || "" });
                }
            } else if (result.repositories && Array.isArray(result.repositories)) {
                for (var i = 0; i < result.repositories.length; i++) {
                    var r = result.repositories[i];
                    drops.push({ user: r.user || "", name: r.name || r.repo || "", description: r.description || "" });
                }
            }
            return drops;
        } catch (e) {
            return [];
        }
    }

    async verifyDrop(user, drop) {
        var result = await this.statDrop(user, drop);
        return result !== null && result.success === true;
    }
}

module.exports = new InkDropClientV2();