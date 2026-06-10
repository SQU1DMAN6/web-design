const authScreen = document.getElementById("auth-screen");
const mainScreen = document.getElementById("main-screen");
const dropsScreen = document.getElementById("drops-screen");
const searchScreen = document.getElementById("search-screen");
const autoMountScreen = document.getElementById("auto-mount-screen");
const logScreen = document.getElementById("log-screen");

const navBtns = document.querySelectorAll(".nav-btn");
const logoutBtn = document.getElementById("logout-btn");

const eventLog = document.getElementById("event-log");
const eventLogAuth = document.getElementById("event-log-auth");
const clearLogBtn = document.getElementById("clear-log-btn");

function appendLog(message, screen) {
    const log = screen === "auth" ? eventLogAuth : eventLog;
    if (!log) return;
    const entry = document.createElement("div");
    entry.className = "log-entry";
    const ts = new Date().toISOString().replace("T", " ").slice(0, 19);
    entry.textContent = ts + " " + message;
    log.prepend(entry);
    if (log.children.length > 500) {
        log.removeChild(log.lastChild);
    }
    console.log("[Inker] " + message);
}

if (clearLogBtn) {
    clearLogBtn.addEventListener("click", function () {
        eventLog.innerHTML = "";
        appendLog("Log cleared");
    });
}

const loginForm = document.getElementById("login-form");
const emailInput = document.getElementById("email");
const passwordInput = document.getElementById("password");
const authError = document.getElementById("auth-error");

function showLoginScreen() {
    appendLog("[UI] showLoginScreen");
    if (authScreen) authScreen.classList.add("active");
    if (mainScreen) mainScreen.classList.remove("active");
    if (emailInput) emailInput.focus();
}

function showMainScreen(user) {
    appendLog("[UI] showMainScreen user=" + (user.username || user.email));
    if (authScreen) authScreen.classList.remove("active");
    if (mainScreen) mainScreen.classList.add("active");
    if (document.getElementById("sidebar-username")) {
        document.getElementById("sidebar-username").textContent = user.username || user.email;
    }
    if (document.getElementById("sidebar-email")) {
        document.getElementById("sidebar-email").textContent = user.email;
    }
    refreshDropsList();
}

if (loginForm) {
    loginForm.addEventListener("submit", async function (e) {
        e.preventDefault();
        appendLog("[UI] login form submitted");
        const email = emailInput.value.trim();
        const password = passwordInput.value;
        if (authError) {
            authError.style.display = "none";
            authError.textContent = "";
        }
        try {
            appendLog("[UI] calling window.inker.login");
            const result = await window.inker.login(email, password);
            appendLog("[UI] login success: " + result.username);
            loginForm.reset();
            showMainScreen(result);
        } catch (err) {
            appendLog("[UI] login error: " + err.message);
            if (authError) {
                authError.textContent = err.message || "Login failed";
                authError.style.display = "block";
            }
        }
    });
}

if (navBtns.length > 0) {
    navBtns.forEach(function (btn) {
        btn.addEventListener("click", function () {
            navBtns.forEach(function (b) { b.classList.remove("active"); });
            btn.classList.add("active");
            const screen = btn.dataset.screen;
            document.querySelectorAll(".main-screen").forEach(function (s) {
                s.classList.remove("active");
            });
            if (screen === "drops" && dropsScreen) { dropsScreen.classList.add("active"); }
            if (screen === "search" && searchScreen) { searchScreen.classList.add("active"); }
            if (screen === "auto-mount" && autoMountScreen) { autoMountScreen.classList.add("active"); }
            if (screen === "log" && logScreen) { logScreen.classList.add("active"); }
        });
    });
}

if (logoutBtn) {
    logoutBtn.addEventListener("click", async function () {
        appendLog("[UI] logout clicked");
        try {
            await window.inker.logout();
            appendLog("[UI] logout done");
            await new Promise(function (resolve) { setTimeout(resolve, 500); });
            showLoginScreen();
        } catch (err) {
            appendLog("[UI] logout error: " + err.message);
        }
    });
}

const dropsList = document.getElementById("drops-list");
const noDropsMsg = document.getElementById("no-drops-message");

function formatSize(bytes) {
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

async function refreshDropsList() {
    appendLog("[UI] refreshDropsList ENTER");
    try {
        const mounts = await window.inker.getActiveMounts();
        const saved = await window.inker.getSavedMounts();
        appendLog("[UI] getActiveMounts returned " + mounts.length + " mounts, saved=" + saved.length);

        if (dropsList) dropsList.innerHTML = "";

        if (mounts.length === 0) {
            if (noDropsMsg) noDropsMsg.style.display = "block";
        } else {
            if (noDropsMsg) noDropsMsg.style.display = "none";

            for (var i = 0; i < mounts.length; i++) {
                var mount = mounts[i];
                var parts = mount.dropPath.split("/");
                var user = parts[0];
                var drop = parts[1];

                // Fetch file index for this drop
                var index = [];
                try {
                    index = await window.inker.getIndex(user, drop);
                } catch (e) {
                    appendLog("[UI] getIndex failed for " + user + "/" + drop + ": " + e.message);
                }

                // Drop card
                var card = document.createElement("div");
                card.className = "drop-card";
                card.innerHTML =
                    '<div class="drop-info">' +
                        '<h3>' + user + '/' + drop + '</h3>' +
                        '<p class="mount-path">' + mount.mountPoint + '</p>' +
                        '<p class="drop-meta">' + index.length + ' item(s)</p>' +
                    '</div>' +
                    '<div class="drop-actions">' +
                        '<button class="btn btn-small open-folder-btn" data-user="' + user + '" data-drop="' + drop + '">Open Folder</button>' +
                        '<button class="menu-btn" data-user="' + user + '" data-drop="' + drop + '">&#x22EE;</button>' +
                    '</div>';
                dropsList.appendChild(card);

                // File list for this drop
                if (index.length > 0) {
                    var fileList = document.createElement("div");
                    fileList.className = "file-list";
                    fileList.setAttribute("data-user", user);
                    fileList.setAttribute("data-drop", drop);

                    for (var j = 0; j < index.length; j++) {
                        var entry = index[j];
                        var isDir = entry.kind === "directory";
                        var item = document.createElement("div");
                        item.className = "file-item" + (isDir ? " file-dir" : " file-file");

                        var icon = isDir ? "&#128193;" : "&#128196;";
                        var sizeStr = isDir ? "" : formatSize(entry.size);
                        var syncedClass = entry.synced ? " synced" : "";

                        item.innerHTML =
                            '<span class="file-icon">' + icon + '</span>' +
                            '<span class="file-name">' + entry.name + '</span>' +
                            '<span class="file-size">' + sizeStr + '</span>' +
                            '<span class="file-status' + syncedClass + '">' + (entry.synced ? "Open" : "Cloud") + '</span>';

                        if (!isDir) {
                            item.setAttribute("data-user", user);
                            item.setAttribute("data-drop", drop);
                            item.setAttribute("data-path", entry.path);
                            item.classList.add("file-downloadable");
                        }

                        fileList.appendChild(item);
                    }
                    dropsList.appendChild(fileList);
                }
            }
        }

        refreshAutoMountList(saved);
    } catch (err) {
        appendLog("[UI] refreshDropsList error: " + err.message);
    }
    appendLog("[UI] refreshDropsList EXIT");
}

// Handle file clicks - download and open
document.addEventListener("click", function (e) {
    var fileItem = e.target.closest(".file-downloadable");
    if (fileItem) {
        var user = fileItem.dataset.user;
        var drop = fileItem.dataset.drop;
        var filePath = fileItem.dataset.path;
        appendLog("[UI] file click: " + filePath);
        downloadAndOpen(user, drop, filePath, fileItem);
    }
});

async function downloadAndOpen(user, drop, relPath, element) {
    appendLog("[UI] downloading " + relPath + "...");
    if (element) {
        element.querySelector(".file-status").textContent = "Loading...";
        element.querySelector(".file-status").className = "file-status";
    }
    try {
        var result = await window.inker.downloadFile(user, drop, relPath);
        appendLog("[UI] downloaded " + relPath + " -> " + result.path);
        await window.inker.openPath(result.path);
        if (element) {
            element.querySelector(".file-status").textContent = "Open";
            element.querySelector(".file-status").className = "file-status synced";
        }
    } catch (err) {
        appendLog("[UI] download error: " + err.message);
        if (element) {
            element.querySelector(".file-status").textContent = "Error";
            element.querySelector(".file-status").className = "file-status error";
        }
    }
}

const contextMenu = document.getElementById("drop-menu");
let activeContextMenu = null;

document.addEventListener("click", function (e) {
    if (e.target.classList.contains("menu-btn")) {
        e.stopPropagation();
        activeContextMenu = {
            user: e.target.dataset.user,
            drop: e.target.dataset.drop
        };
        const rect = e.target.getBoundingClientRect();
        if (contextMenu) {
            contextMenu.classList.remove("hidden");
            contextMenu.style.left = Math.max(10, rect.left - 150) + "px";
            contextMenu.style.top = Math.max(10, rect.bottom + 6) + "px";
        }
    } else if (!contextMenu || !contextMenu.contains(e.target)) {
        if (contextMenu) contextMenu.classList.add("hidden");
    }
});

const menuAddAuto = document.getElementById("menu-add-automount");
if (menuAddAuto) {
    menuAddAuto.addEventListener("click", async function () {
        if (!activeContextMenu) return;
        appendLog("[UI] menu-add-automount for " + activeContextMenu.user + "/" + activeContextMenu.drop);
        try {
            await window.inker.setAutoMount(activeContextMenu.user, activeContextMenu.drop, true);
            appendLog("[UI] auto-add enabled");
            if (contextMenu) contextMenu.classList.add("hidden");
            const saved = await window.inker.getSavedMounts();
            refreshAutoMountList(saved);
        } catch (err) {
            appendLog("[UI] auto-add error: " + err.message);
        }
    });
}

const menuRemove = document.getElementById("menu-remove");
if (menuRemove) {
    menuRemove.addEventListener("click", async function () {
        if (!activeContextMenu) return;
        appendLog("[UI] menu-remove for " + activeContextMenu.user + "/" + activeContextMenu.drop);
        try {
            await window.inker.unmountDrop(activeContextMenu.user, activeContextMenu.drop);
            appendLog("[UI] remove done");
            if (contextMenu) contextMenu.classList.add("hidden");
            refreshDropsList();
        } catch (err) {
            appendLog("[UI] remove error: " + err.message);
        }
    });
}

const menuOpen = document.getElementById("menu-open");
if (menuOpen) {
    menuOpen.addEventListener("click", async function () {
        if (!activeContextMenu) return;
        appendLog("[UI] menu-open for " + activeContextMenu.user + "/" + activeContextMenu.drop);
        try {
            const mounts = await window.inker.getActiveMounts();
            const mount = mounts.find(function (m) {
                return m.dropPath === activeContextMenu.user + "/" + activeContextMenu.drop;
            });
            if (mount) {
                appendLog("[UI] opening folder: " + mount.mountPoint);
                await window.inker.openPath(mount.mountPoint);
            }
            if (contextMenu) contextMenu.classList.add("hidden");
        } catch (err) {
            appendLog("[UI] open error: " + err.message);
        }
    });
}

document.addEventListener("click", function (e) {
    const openBtn = e.target.closest(".open-folder-btn");
    if (openBtn) {
        activeContextMenu = { user: openBtn.dataset.user, drop: openBtn.dataset.drop };
        if (menuOpen) menuOpen.click();
    }
});

const searchInput = document.getElementById("search-input");
const searchBtn = document.getElementById("search-btn");
const searchResults = document.getElementById("search-results");
const noSearchMsg = document.getElementById("no-search-message");

if (searchBtn && searchInput) {
    searchBtn.addEventListener("click", async function () {
        const query = searchInput.value.trim();
        if (!query) return;
        appendLog("[UI] search for: " + query);
        try {
            const results = await window.inker.searchDrops(query);
            appendLog("[UI] search returned " + results.length + " result(s)");
            if (searchResults) searchResults.innerHTML = "";
            if (noSearchMsg) noSearchMsg.style.display = results.length === 0 ? "block" : "none";
            results.forEach(function (drop) {
                if (searchResults) {
                    const card = document.createElement("div");
                    card.className = "search-result-card";
                    card.innerHTML =
                        '<h4>' + (drop.user || "?") + '/' + (drop.name || "?") + '</h4>' +
                        '<p>' + (drop.description || "No description") + '</p>' +
                        '<button class="btn btn-small btn-primary add-from-search" data-user="' + drop.user + '" data-drop="' + drop.name + '">Add</button>';
                    searchResults.appendChild(card);
                }
            });
        } catch (err) {
            appendLog("[UI] search error: " + err.message);
        }
    });
    searchInput.addEventListener("keypress", function (e) {
        if (e.key === "Enter") searchBtn.click();
    });
}

document.addEventListener("click", function (e) {
    const addBtn = e.target.closest(".add-from-search");
    if (addBtn) {
        const user = addBtn.dataset.user;
        const drop = addBtn.dataset.drop;
        appendLog("[UI] add-from-search clicked: " + user + "/" + drop);
        addDrop(user, drop);
    }
});

const addByPathInput = document.getElementById("add-by-path-input");
const addByPathBtn = document.getElementById("add-by-path-btn");

async function addDrop(user, drop) {
    appendLog("[UI] addDrop ENTER: " + user + "/" + drop);
    try {
        appendLog("[UI] addDrop calling window.inker.mountDrop");
        const result = await window.inker.mountDrop(user, drop, null);
        appendLog("[UI] addDrop success, result: " + JSON.stringify(result));
        await refreshDropsList();
    } catch (err) {
        appendLog("[UI] addDrop error: " + err.message);
    }
    appendLog("[UI] addDrop EXIT");
}

if (addByPathBtn && addByPathInput) {
    addByPathBtn.addEventListener("click", async function () {
        var path = addByPathInput.value.trim();
        if (!path) return;
        var parts = path.split("/");
        if (parts.length !== 2 || !parts[0] || !parts[1]) {
            appendLog("[UI] invalid drop path: " + path);
            return;
        }
        appendLog("[UI] add-by-path: " + parts[0] + "/" + parts[1]);
        addDrop(parts[0], parts[1]);
    });
    addByPathInput.addEventListener("keypress", function (e) {
        if (e.key === "Enter") addByPathBtn.click();
    });
}

function refreshAutoMountList(mounts) {
    appendLog("[UI] refreshAutoMountList ENTER");
    const autoMountList = document.getElementById("auto-mount-list");
    if (!autoMountList) return;
    autoMountList.innerHTML = "";
    const autoMounts = mounts.filter(function (m) { return m.auto_mount; });
    if (autoMounts.length === 0) {
        autoMountList.innerHTML = '<p class="empty-state">No Drops set for auto add.</p>';
        return;
    }
    autoMounts.forEach(function (mount) {
        const item = document.createElement("div");
        item.className = "auto-mount-item";
        item.innerHTML =
            '<span>' + mount.drop_path + ' &#x2192; ' + mount.mount_point + '</span>' +
            '<button class="btn btn-small" data-drop-path="' + mount.drop_path + '">Disable</button>';
        autoMountList.appendChild(item);
    });
    document.querySelectorAll(".auto-mount-item button").forEach(function (btn) {
        btn.addEventListener("click", async function () {
            const dropPath = btn.dataset.dropPath;
            const parts = dropPath.split("/");
            try {
                await window.inker.setAutoMount(parts[0], parts[1], false);
                const saved = await window.inker.getSavedMounts();
                refreshAutoMountList(saved);
                appendLog("[UI] auto-add disabled for " + dropPath);
            } catch (err) {
                appendLog("[UI] auto-add disable error: " + err.message);
            }
        });
    });
    appendLog("[UI] refreshAutoMountList EXIT");
}

const minBtn = document.getElementById("min-btn");
if (minBtn) minBtn.addEventListener("click", function () { window.inker.minimize(); });

const maxBtn = document.getElementById("max-btn");
if (maxBtn) maxBtn.addEventListener("click", function () { window.inker.maximize(); });

const closeBtn = document.getElementById("close-btn");
if (closeBtn) closeBtn.addEventListener("click", function () { window.inker.close(); });

if (window.inker) {
    window.inker.onLog(function (message) {
        appendLog(message);
    });
    window.inker.onReady(async function () {
        appendLog("[UI] onReady");
        const user = await window.inker.getCurrentUser();
        if (user) {
            showMainScreen(user);
            appendLog("[UI] session restored for " + user.username);
        } else {
            showLoginScreen();
        }
    });
}

if (window.inker) {
    window.inker.getCurrentUser().then(function (user) {
        if (user) {
            showMainScreen(user);
            appendLog("[UI] session loaded for " + user.username);
        } else {
            showLoginScreen();
        }
    }).catch(function () {
        showLoginScreen();
    });
}