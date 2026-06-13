const https = require("https");
const http = require("http");
const { URL } = require("url");

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

    _makeRequest(method, path, body, isJson) {
        var self = this;
        return new Promise(function (resolve, reject) {
            var url = new URL(path.startsWith("http") ? path : self.baseURL + path);
            var isHttps = url.protocol === "https:";
            var client = isHttps ? https : http;

            var ts = new Date().toISOString().replace("T", " ").slice(0, 19);
            console.log("[API REQUEST] " + ts + " " + method + " " + path);

            var options = {
                method: method,
                headers: {
                    "User-Agent": "FTR-Inker-3.2",
                    "X-FTR-CLIENT": "FtR-Inker-3.2"
                }
            };

            if (isJson) {
                options.headers["Content-Type"] = "application/json";
            } else if (body && method !== "GET") {
                options.headers["Content-Type"] = "application/x-www-form-urlencoded";
            }

            if (self.sessionId) {
                options.headers["Cookie"] = "PHPSESSID=" + self.sessionId;
            }

            if (body && method !== "GET") {
                options.headers["Content-Length"] = Buffer.byteLength(body);
            }

            var req = client.request(url, options, function (res) {
                console.log("[API RESPONSE] " + ts + " " + method + " " + path + " -> " + res.statusCode);

                var setCookie = res.headers["set-cookie"];
                if (setCookie) {
                    for (var ci = 0; ci < setCookie.length; ci++) {
                        var cookieStr = setCookie[ci];
                        var parts = cookieStr.split(";");
                        if (parts.length > 0) {
                            var nv = parts[0].split("=");
                            if (nv.length === 2 && nv[0] === "PHPSESSID") {
                                self.sessionId = nv[1];
                                console.log("[API] " + ts + " Captured session ID from Set-Cookie: " + self.sessionId.substring(0, 16) + "...");
                            }
                        }
                    }
                }

                var data = "";
                res.on("data", function (chunk) { data += chunk; });
                res.on("end", function () {
                    if (res.statusCode >= 400) {
                        console.log("[API ERROR] " + ts + " " + method + " " + path + " HTTP " + res.statusCode + ": " + data.substring(0, 500));
                        reject(new Error("HTTP " + res.statusCode + ": " + data.substring(0, 500)));
                    } else {
                        try {
                            if (isJson && data) {
                                var parsed = JSON.parse(data);
                                console.log("[API OK] " + ts + " " + method + " " + path + " -> JSON keys: " + Object.keys(parsed).join(", "));
                                resolve(parsed);
                            } else {
                                console.log("[API OK] " + ts + " " + method + " " + path + " -> text length=" + data.length);
                                resolve(data);
                            }
                        } catch (e) {
                            console.log("[API OK] " + ts + " " + method + " " + path + " -> text fallback length=" + data.length);
                            resolve(data);
                        }
                    }
                });
            });

            req.on("error", function (err) {
                console.log("[API ERROR] " + ts + " " + method + " " + path + " network error: " + err.message);
                reject(err);
            });

            if (body && method !== "GET") {
                req.write(body);
            }
            req.end();
        });
    }

    _parseMatches(result) {
        var drops = [];
        if (result.matches && Array.isArray(result.matches)) {
            for (var i = 0; i < result.matches.length; i++) {
                var m = result.matches[i];
                drops.push({
                    user: m.user || "",
                    name: m.repo || m.name || "",
                    description: m.description || ""
                });
            }
        } else if (result.repositories && Array.isArray(result.repositories)) {
            for (var i = 0; i < result.repositories.length; i++) {
                var r = result.repositories[i];
                drops.push({
                    user: r.user || "",
                    name: r.name || r.repo || "",
                    description: r.description || ""
                });
            }
        }
        return drops;
    }

    async _listAllDrops() {
        var ts = new Date().toISOString().replace("T", " ").slice(0, 19);
        console.log("[API] " + ts + " Fetching all Drops (session=" + (this.sessionId ? "yes" : "no") + ")");
        try {
            var result = await this._makeRequest("GET", "/index.php?search=%2F&api=1", null, true);
            var drops = this._parseMatches(result);
            console.log("[API] " + ts + " Total Drops: " + drops.length);
            return drops;
        } catch (e) {
            console.log("[API] " + ts + " Fetch all failed: " + e.message);
            return [];
        }
    }

    async login(email, password) {
        var ts = new Date().toISOString().replace("T", " ").slice(0, 19);
        console.log("[API] " + ts + " Login attempt for " + email);
        var body = "email=" + encodeURIComponent(email) + "&password=" + encodeURIComponent(password);

        this.email = email;

        var response = await this._makeRequest("POST", "/login.php", body);

        if (response.includes("Error logging in")) {
            this.sessionId = null;
            console.log("[API] " + ts + " Login failed: invalid credentials");
            throw new Error("Invalid credentials");
        }

        var userMatch = response.match(/Logged in as <b>([^<]+)<\/b>/);
        var username = userMatch ? userMatch[1] : email;

        this.username = username;

        console.log("[API] " + ts + " Login successful for " + username + " session=" + (this.sessionId ? "yes" : "no"));

        return { email: email, username: username };
    }

    async searchDrops(query) {
        var ts = new Date().toISOString().replace("T", " ").slice(0, 19);
        var path = "/index.php?search=" + encodeURIComponent(query) + "&api=1";
        console.log("[API] " + ts + " Searching Drops for: " + query + " (session=" + (this.sessionId ? "yes" : "no") + ")");
        try {
            var result = await this._makeRequest("GET", path, null, true);
            var drops = this._parseMatches(result);
            console.log("[API] " + ts + " Server returned " + drops.length + " matches");

            console.log("[API] " + ts + " Returning " + drops.length + " Drop(s)");
            return drops;
        } catch (e) {
            console.log("[API] " + ts + " Search failed: " + e.message);
            return [];
        }
    }

    async listDrops() {
        return await this._listAllDrops();
    }

    async verifyDrop(user, drop) {
        var ts = new Date().toISOString().replace("T", " ").slice(0, 19);
        var path = "/api/fs/" + encodeURIComponent(user) + "/" + encodeURIComponent(drop) + "?list=1";
        console.log("[API] " + ts + " Verifying Drop " + user + "/" + drop);
        try {
            var result = await this._makeRequest("GET", path, null, true);
            var exists = result.success === true;
            console.log("[API] " + ts + " Drop " + user + "/" + drop + " exists: " + exists);
            return exists;
        } catch (e) {
            console.log("[API] " + ts + " Drop " + user + "/" + drop + " not found: " + e.message);
            return false;
        }
    }

    async getFileList(user, drop) {
        var ts = new Date().toISOString().replace("T", " ").slice(0, 19);
        var path = "/api/fs/" + encodeURIComponent(user) + "/" + encodeURIComponent(drop) + "?list=1";
        console.log("[API] " + ts + " Listing files for " + user + "/" + drop);
        try {
            var result = await this._makeRequest("GET", path, null, true);
            if (!result.success) {
                throw new Error(result.error || "Failed to list files");
            }
            console.log("[API] " + ts + " Found " + (result.entries || []).length + " entries");
            return result.entries || [];
        } catch (e) {
            console.log("[API] " + ts + " List files failed for " + user + "/" + drop + ": " + e.message);
            return [];
        }
    }

    async downloadFile(user, drop, filePath) {
        var ts = new Date().toISOString().replace("T", " ").slice(0, 19);
        var encodedPath = filePath.split("/").map(function (p) { return encodeURIComponent(p); }).join("/");
        var path = "/api/fs/" + encodeURIComponent(user) + "/" + encodeURIComponent(drop) + "/" + encodedPath;
        return new Promise(function (resolve, reject) {
            var self = this;
            var url = new URL(self.baseURL + path);
            var isHttps = url.protocol === "https:";
            var client = isHttps ? https : http;

            var options = {
                headers: {
                    "User-Agent": "FTR-Inker-3.2",
                    "X-FTR-CLIENT": "FtR-Inker-3.2"
                }
            };

            if (self.sessionId) {
                options.headers["Cookie"] = "PHPSESSID=" + self.sessionId;
            }

            var req = client.request(url, options, function (res) {
                console.log("[API] " + ts + " Download response: " + res.statusCode);
                if (res.statusCode !== 200) {
                    reject(new Error("HTTP " + res.statusCode));
                } else {
                    resolve(res);
                }
            });

            req.on("error", function (err) {
                console.log("[API] " + ts + " Download network error: " + err.message);
                reject(err);
            });
            req.end();
        }.bind(this));
    }

    async uploadFile(user, drop, filePath, fileStream, size) {
        var ts = new Date().toISOString().replace("T", " ").slice(0, 19);
        var encodedPath = filePath.split("/").map(function (p) { return encodeURIComponent(p); }).join("/");
        var path = "/api/fs/" + encodeURIComponent(user) + "/" + encodeURIComponent(drop) + "/" + encodedPath;
        return new Promise(function (resolve, reject) {
            var self = this;
            var url = new URL(self.baseURL + path);
            var isHttps = url.protocol === "https:";
            var client = isHttps ? https : http;

            var options = {
                method: "PUT",
                headers: {
                    "User-Agent": "FTR-Inker-3.2",
                    "X-FTR-CLIENT": "FtR-Inker-3.2",
                    "Content-Length": size
                }
            };

            if (self.sessionId) {
                options.headers["Cookie"] = "PHPSESSID=" + self.sessionId;
            }

            var req = client.request(url, options, function (res) {
                var data = "";
                res.on("data", function (chunk) { data += chunk; });
                res.on("end", function () {
                    if (res.statusCode >= 400) {
                        reject(new Error("HTTP " + res.statusCode + ": " + data));
                    } else {
                        try {
                            resolve(JSON.parse(data));
                        } catch (e) {
                            resolve({ success: true });
                        }
                    }
                });
            });

            req.on("error", function (err) {
                console.log("[API] " + ts + " Upload network error: " + err.message);
                reject(err);
            });
            fileStream.pipe(req);
        }.bind(this));
    }

    async deleteFile(user, drop, filePath) {
        var result = await this._makeRequest("DELETE",
            "/api/fs/" + encodeURIComponent(user) + "/" + encodeURIComponent(drop) + "/" +
            filePath.split("/").map(function (p) { return encodeURIComponent(p); }).join("/"),
            null, true);
        if (!result.success) throw new Error(result.error || "Failed to delete file");
        return result;
    }

    async sessionConfirm() {
        var ts = new Date().toISOString().replace("T", " ").slice(0, 19);
        console.log("[API] " + ts + " Session confirmation check...");
        try {
            var result = await this._makeRequest("GET", "/sessionconfirm", null, true);
            if (result && result.success === true) {
                if (result.username) {
                    this.username = result.username;
                }
                console.log("[API] " + ts + " Session confirmed: " + (this.username || "yes"));
                return {
                    valid: true,
                    email: this.email,
                    username: this.username || result.username
                };
            }
            console.log("[API] " + ts + " Session not valid");
            return { valid: false };
        } catch (e) {
            console.log("[API] " + ts + " Session confirm error: " + e.message);
            return { valid: false };
        }
    }

    async createDirectory(user, drop, dirPath) {
        var result = await this._makeRequest("POST",
            "/api/fs/" + encodeURIComponent(user) + "/" + encodeURIComponent(drop) + "/" +
            dirPath.split("/").map(function (p) { return encodeURIComponent(p); }).join("/") + "?op=mkdir",
            JSON.stringify({}), true);
        if (!result.success) throw new Error(result.error || "Failed to create directory");
        return result;
    }

    async renameFile(user, drop, oldPath, newPath) {
        var result = await this._makeRequest("POST",
            "/api/fs/" + encodeURIComponent(user) + "/" + encodeURIComponent(drop) + "/" +
            oldPath.split("/").map(function (p) { return encodeURIComponent(p); }).join("/") + "?op=rename",
            JSON.stringify({ newPath: newPath }), true);
        if (!result.success) throw new Error(result.error || "Failed to rename file");
        return result;
    }
}

module.exports = new InkDropClient();