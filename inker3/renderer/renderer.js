// ====== Auth Screen ======
const authScreen = document.getElementById("auth-screen");
const mainScreen = document.getElementById("main-screen");
const loginForm = document.getElementById("login-form");
const emailInput = document.getElementById("email");
const passwordInput = document.getElementById("password");
const authError = document.getElementById("auth-error");
const registerLink = document.getElementById("register-link");

// ====== Main Screen ======
const navBtns = document.querySelectorAll(".nav-btn");
const logoutBtn = document.getElementById("logout-btn");
const dropsScreen = document.getElementById("drops-screen");
const autoMountScreen = document.getElementById("auto-mount-screen");
const searchScreen = document.getElementById("search-screen");
const logScreen = document.getElementById("log-screen");

// ====== Drops Panel ======
const dropsList = document.getElementById("drops-list");
const noDropsMsg = document.getElementById("no-drops-message");

// ====== Auto-Mount Panel ======
const autoMountList = document.getElementById("auto-mount-list");
const noAutoMountMsg = document.getElementById("no-auto-mount-message");

// ====== Search Panel ======
const searchInput = document.getElementById("search-input");
const searchBtn = document.getElementById("search-btn");
const addByPathInput = document.getElementById("add-by-path-input");
const addByPathBtn = document.getElementById("add-by-path-btn");
const searchResults = document.getElementById("search-results");
const noSearchMsg = document.getElementById("no-search-message");
const searchStatus = document.getElementById("search-status");

// ====== Log Panel ======
const eventLog = document.getElementById("event-log");
const clearLogBtn = document.getElementById("clear-log-btn");

// ====== Window Controls ======
const minBtn = document.getElementById("min-btn");
const maxBtn = document.getElementById("max-btn");
const closeBtn = document.getElementById("close-btn");

// ====== Context Menu ======
const dropMenu = document.getElementById("drop-menu");
const menuOpen = document.getElementById("menu-open");
const menuAddAuto = document.getElementById("menu-add-automount");
const menuRemoveAuto = document.getElementById("menu-remove-automount");
const menuRemove = document.getElementById("menu-remove");

// ====== Cache ======
const cacheInfo = document.getElementById("cache-info");
const clearCacheBtn = document.getElementById("clear-cache-btn");

// ====== State ======
var activeContextDrop = null;
var authData = null;
var sessionCheckDone = false;

// ====== Log Helper ======
function appendLog(message) {
    var log = authScreen.classList.contains("active") ? document.getElementById("event-log-auth") : eventLog;
    if (!log) log = eventLog;
    if (!log) return;
    var entry = document.createElement("div");
    entry.className = "log-entry";
    var ts = new Date().toISOString().replace("T", " ").slice(0, 19);
    entry.textContent = ts + " " + message;
    log.prepend(entry);
    if (log.children.length > 500) {
        log.removeChild(log.lastChild);
    }
    console.log("[Inker] " + message);
}

// ====== File Icon Helper ======
function getFileIcon(filename) {
    if (!filename) return "\uD83D\uDCC4";
    var ext = filename.split(".").pop().toLowerCase();
    var icons = {
        "mp3":"\uD83C\uDFB5","wav":"\uD83C\uDFB5","flac":"\uD83C\uDFB5","ogg":"\uD83C\uDFB5","aac":"\uD83C\uDFB5","m4a":"\uD83C\uDFB5",
        "mp4":"\uD83C\uDFAC","avi":"\uD83C\uDFAC","mkv":"\uD83C\uDFAC","mov":"\uD83C\uDFAC","webm":"\uD83C\uDFAC",
        "jpg":"\uD83D\uDDBC","jpeg":"\uD83D\uDDBC","png":"\uD83D\uDDBC","gif":"\uD83D\uDDBC","bmp":"\uD83D\uDDBC","webp":"\uD83D\uDDBC","svg":"\uD83D\uDDBC",
        "pdf":"\uD83D\uDCC4","doc":"\uD83D\uDCDD","docx":"\uD83D\uDCDD","xls":"\uD83D\uDCCA","xlsx":"\uD83D\uDCCA","ppt":"\uD83D\uDCC8","pptx":"\uD83D\uDCC8",
        "zip":"\uD83D\uDCE6","rar":"\uD83D\uDCE6","7z":"\uD83D\uDCE6","tar":"\uD83D\uDCE6","gz":"\uD83D\uDCE6",
        "txt":"\uD83D\uDCDD","md":"\uD83D\uDCDD","json":"\uD83D\uDCCB","xml":"\uD83D\uDCCB",
        "exe":"\u2699","msi":"\u2699","dll":"\u2699",
        "js":"\uD83D\uDCD8","ts":"\uD83D\uDCD8","py":"\uD83D\uDCD8","go":"\uD83D\uDCD8","c":"\uD83D\uDCD8","cpp":"\uD83D\uDCD8","java":"\uD83D\uDCD8",
        "html":"\uD83C\uDF10","css":"\uD83C\uDFA8","scss":"\uD83C\uDFA8","less":"\uD83C\uDFA8",
        "folder":"\uD83D\uDCC1","directory":"\uD83D\uDCC1"
    };
    return icons[ext] || "\uD83D\uDCC4";
}

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

// ====== Screen Transitions ======
function showLoginScreen() {
    authScreen.classList.add("active");
    mainScreen.classList.remove("active");
    sessionCheckDone = true;
    if (emailInput) emailInput.focus();
}

function showMainScreen(user) {
    authData = user;
    authScreen.classList.remove("active");
    mainScreen.classList.add("active");
    sessionCheckDone = true;
    document.getElementById("sidebar-username").textContent = user.username || user.email;
    document.getElementById("sidebar-email").textContent = user.email;
    refreshDropsList();
    refreshAutoMountList();
    refreshCacheInfo();
}

// ====== Navigation ======
if (navBtns.length > 0) {
    navBtns.forEach(function (btn) {
        btn.addEventListener("click", function () {
            navBtns.forEach(function (b) { b.classList.remove("active"); });
            btn.classList.add("active");
            var screen = btn.dataset.screen;
            document.querySelectorAll(".main-screen").forEach(function (s) {
                s.classList.remove("active");
            });
            if (screen === "drops" && dropsScreen) { dropsScreen.classList.add("active"); refreshDropsList(); }
            if (screen === "auto-mount" && autoMountScreen) { autoMountScreen.classList.add("active"); refreshAutoMountList(); }
            if (screen === "search" && searchScreen) { searchScreen.classList.add("active"); }
            if (screen === "log" && logScreen) { logScreen.classList.add("active"); }
        });
    });
}

// ====== Login ======
if (loginForm) {
    loginForm.addEventListener("submit", async function (e) {
        e.preventDefault();
        var email = emailInput.value.trim();
        var password = passwordInput.value;
        if (authError) { authError.style.display = "none"; authError.textContent = ""; }
        try {
            appendLog("Logging in as " + email + "...");
            var result = await window.inker.login(email, password);
            appendLog("Login successful: " + result.username);
            loginForm.reset();
            showMainScreen(result);
        } catch (err) {
            appendLog("Login error: " + err.message);
            if (authError) { authError.textContent = err.message || "Login failed"; authError.style.display = "block"; }
        }
    });
}

if (registerLink) {
    registerLink.addEventListener("click", function (e) {
        e.preventDefault();
        window.inker.openExternal("https://inkdrop.quanthai.net/register");
    });
}

// ====== Logout ======
if (logoutBtn) {
    logoutBtn.addEventListener("click", async function () {
        appendLog("Logging out...");
        try {
            await window.inker.logout();
            appendLog("Logged out");
            authData = null;
            showLoginScreen();
        } catch (err) { appendLog("Logout error: " + err.message); }
    });
}

// ====== Cache Info ======
async function refreshCacheInfo() {
    if (!cacheInfo) return;
    try {
        var info = await window.inker.getCacheInfo();
        if (info && info.size > 0) {
            cacheInfo.textContent = "Cached files: " + formatSize(info.size);
        } else {
            cacheInfo.textContent = "No cached files";
        }
    } catch (e) {
        cacheInfo.textContent = "Cache: unknown";
    }
}

if (clearCacheBtn) {
    clearCacheBtn.addEventListener("click", async function () {
        try {
            await window.inker.clearCache();
            appendLog("Cache cleared");
            refreshCacheInfo();
        } catch (err) {
            appendLog("Clear cache error: " + err.message);
        }
    });
}

// ====== Drops List (no file browser — just mount cards) ======
async function refreshDropsList() {
    try {
        var mounts = await window.inker.getActiveMounts();
        if (dropsList) dropsList.innerHTML = "";
        if (noDropsMsg) noDropsMsg.style.display = mounts.length === 0 ? "block" : "none";

        for (var i = 0; i < mounts.length; i++) {
            var mount = mounts[i];
            var parts = mount.dropPath.split("/");
            var user = parts[0];
            var drop = parts[1];

            var section = document.createElement("div");
            section.className = "mount-section";

            var card = document.createElement("div");
            card.className = "drop-card";
            card.innerHTML =
                '<div class="drop-info">' +
                    '<h3>' + user + '/' + drop + '</h3>' +
                    '<p class="mount-path">' + mount.mountPoint + '</p>' +
                '</div>' +
                '<div class="drop-actions">' +
                    '<button class="btn btn-small open-folder-btn" data-user="' + user + '" data-drop="' + drop + '">Open</button>' +
                    '<button class="btn btn-small menu-btn" data-user="' + user + '" data-drop="' + drop + '">...</button>' +
                '</div>';
            section.appendChild(card);
            dropsList.appendChild(section);
        }
    } catch (err) {
        appendLog("refreshDropsList error: " + err.message);
    }
}

function escapeHtml(str) {
    if (!str) return "";
    return String(str).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

// ====== Auto-Mount List ======
async function refreshAutoMountList() {
    try {
        var saved = await window.inker.getSavedMounts();
        if (autoMountList) autoMountList.innerHTML = "";
        var autoMounts = saved.filter(function (m) { return m.auto_mount; });
        if (noAutoMountMsg) noAutoMountMsg.style.display = autoMounts.length === 0 ? "block" : "none";

        for (var i = 0; i < autoMounts.length; i++) {
            var m = autoMounts[i];
            var item = document.createElement("div");
            item.className = "auto-mount-item";
            item.innerHTML =
                '<span class="auto-mount-path">' + m.drop_path + '</span>' +
                '<button class="btn btn-small btn-ghost disable-auto-btn" data-drop-path="' + m.drop_path + '">Disable</button>';
            autoMountList.appendChild(item);
        }

        document.querySelectorAll(".disable-auto-btn").forEach(function (btn) {
            btn.addEventListener("click", async function () {
                var dropPath = btn.dataset.dropPath;
                var parts = dropPath.split("/");
                try {
                    await window.inker.setAutoMount(parts[0], parts[1], false);
                    refreshAutoMountList();
                    refreshDropsList();
                    appendLog("Auto-add disabled for " + dropPath);
                } catch (err) { appendLog("Disable auto-add error: " + err.message); }
            });
        });
    } catch (err) { appendLog("refreshAutoMountList error: " + err.message); }
}

// ====== Context Menu ======
document.addEventListener("click", function (e) {
    if (e.target.classList.contains("menu-btn")) {
        e.stopPropagation();
        var user = e.target.dataset.user;
        var drop = e.target.dataset.drop;
        activeContextDrop = { user: user, drop: drop };

        (async function () {
            try {
                var saved = await window.inker.getSavedMounts();
                var mount = saved.find(function (m) { return m.drop_path === user + "/" + drop; });
                if (mount && mount.auto_mount) {
                    if (menuAddAuto) menuAddAuto.style.display = "none";
                    if (menuRemoveAuto) menuRemoveAuto.style.display = "block";
                } else {
                    if (menuAddAuto) menuAddAuto.style.display = "block";
                    if (menuRemoveAuto) menuRemoveAuto.style.display = "none";
                }
            } catch (e) {
                if (menuAddAuto) menuAddAuto.style.display = "block";
                if (menuRemoveAuto) menuRemoveAuto.style.display = "none";
            }
        })();

        var rect = e.target.getBoundingClientRect();
        if (dropMenu) {
            dropMenu.classList.remove("hidden");
            dropMenu.style.left = Math.max(10, rect.left - 150) + "px";
            dropMenu.style.top = Math.max(10, rect.bottom + 6) + "px";
        }
        return;
    }

    var openBtn = e.target.closest(".open-folder-btn");
    if (openBtn) {
        e.stopPropagation();
        activeContextDrop = { user: openBtn.dataset.user, drop: openBtn.dataset.drop };
        openDropFolder();
        return;
    }

    if (dropMenu && !dropMenu.contains(e.target)) {
        dropMenu.classList.add("hidden");
    }
});

async function openDropFolder() {
    if (!activeContextDrop) return;
    try {
        var mounts = await window.inker.getActiveMounts();
        var mount = mounts.find(function (m) {
            return m.dropPath === activeContextDrop.user + "/" + activeContextDrop.drop;
        });
        if (mount) {
            appendLog("Opening folder: " + mount.mountPoint);
            await window.inker.openPath(mount.mountPoint);
        }
    } catch (err) { appendLog("Open folder error: " + err.message); }
    if (dropMenu) dropMenu.classList.add("hidden");
}

if (menuOpen) { menuOpen.addEventListener("click", openDropFolder); }
if (menuAddAuto) {
    menuAddAuto.addEventListener("click", async function () {
        if (!activeContextDrop) return;
        try {
            await window.inker.setAutoMount(activeContextDrop.user, activeContextDrop.drop, true);
            appendLog("Auto-add enabled for " + activeContextDrop.user + "/" + activeContextDrop.drop);
            if (dropMenu) dropMenu.classList.add("hidden");
            refreshAutoMountList();
        } catch (err) { appendLog("Auto-add error: " + err.message); }
    });
}
if (menuRemoveAuto) {
    menuRemoveAuto.addEventListener("click", async function () {
        if (!activeContextDrop) return;
        try {
            await window.inker.setAutoMount(activeContextDrop.user, activeContextDrop.drop, false);
            appendLog("Auto-add disabled for " + activeContextDrop.user + "/" + activeContextDrop.drop);
            if (dropMenu) dropMenu.classList.add("hidden");
            refreshAutoMountList();
        } catch (err) { appendLog("Auto-add disable error: " + err.message); }
    });
}
if (menuRemove) {
    menuRemove.addEventListener("click", async function () {
        if (!activeContextDrop) return;
        appendLog("Removing " + activeContextDrop.user + "/" + activeContextDrop.drop + "...");
        try {
            await window.inker.unmountDrop(activeContextDrop.user, activeContextDrop.drop);
            appendLog("Removed " + activeContextDrop.user + "/" + activeContextDrop.drop);
            if (dropMenu) dropMenu.classList.add("hidden");
            refreshDropsList();
            refreshAutoMountList();
        } catch (err) { appendLog("Remove error: " + err.message); }
    });
}

// ====== Search ======
if (searchBtn && searchInput) {
    searchBtn.addEventListener("click", async function () {
        var query = searchInput.value.trim();
        if (!query) return;
        if (searchResults) searchResults.innerHTML = "";
        if (noSearchMsg) noSearchMsg.style.display = "none";
        if (searchStatus) searchStatus.style.display = "none";
        appendLog("Searching for: " + query);
        try {
            var results = await window.inker.searchDrops(query);
            if (searchStatus) {
                searchStatus.textContent = "Found " + results.length + " result(s)";
                searchStatus.className = "status-message status-info";
                searchStatus.style.display = "block";
            }
            if (results.length === 0) {
                if (noSearchMsg) noSearchMsg.style.display = "block";
                return;
            }
            results.forEach(function (drop) {
                var item = document.createElement("div");
                item.className = "drop-card search-result-card";
                item.innerHTML =
                    '<div class="drop-info">' +
                        '<h3>' + (drop.user || "?") + '/' + (drop.name || "?") + '</h3>' +
                        '<p class="drop-desc">' + (drop.description || "No description") + '</p>' +
                    '</div>' +
                    '<div class="drop-actions">' +
                        '<button class="btn btn-primary btn-small add-from-search" data-user="' + drop.user + '" data-drop="' + drop.name + '">Add</button>' +
                    '</div>';
                if (searchResults) searchResults.appendChild(item);
            });
        } catch (err) {
            appendLog("Search error: " + err.message);
            if (searchStatus) {
                searchStatus.textContent = "Search failed: " + err.message;
                searchStatus.className = "status-message status-error";
                searchStatus.style.display = "block";
            }
        }
    });
    if (searchInput) {
        searchInput.addEventListener("keypress", function (e) {
            if (e.key === "Enter" && searchBtn) searchBtn.click();
        });
    }
}

// Add from search results
document.addEventListener("click", function (e) {
    var addBtn = e.target.closest(".add-from-search");
    if (addBtn) {
        addBtn.textContent = "Adding...";
        addBtn.disabled = true;
        addDrop(addBtn.dataset.user, addBtn.dataset.drop, addBtn);
    }
});

// ====== Add Drop (Mount) ======
async function addDrop(user, drop, button) {
    appendLog("Adding " + user + "/" + drop + "...");
    try {
        await window.inker.mountDrop(user, drop, null);
        appendLog("Added " + user + "/" + drop);
        if (button) {
            button.textContent = "Added";
            button.className = "btn btn-small btn-added";
            button.disabled = false;
        }
        refreshDropsList();
        refreshAutoMountList();
    } catch (err) {
        appendLog("Add error: " + err.message);
        if (button) { button.textContent = "Add"; button.disabled = false; }
    }
}

// ====== Add by path ======
if (addByPathBtn && addByPathInput) {
    addByPathBtn.addEventListener("click", async function () {
        var pathInput = addByPathInput.value.trim();
        if (!pathInput) return;
        var parts = pathInput.split("/");
        if (parts.length !== 2 || !parts[0] || !parts[1]) {
            appendLog("Invalid drop path: " + pathInput);
            return;
        }
        appendLog("Adding by path: " + parts[0] + "/" + parts[1]);
        addByPathBtn.textContent = "Adding...";
        addByPathBtn.disabled = true;
        await addDrop(parts[0], parts[1], addByPathBtn);
        addByPathBtn.textContent = "Add";
        addByPathBtn.disabled = false;
        addByPathInput.value = "";
    });
    if (addByPathInput) {
        addByPathInput.addEventListener("keypress", function (e) {
            if (e.key === "Enter" && addByPathBtn) addByPathBtn.click();
        });
    }
}

// ====== Window Controls ======
if (minBtn) minBtn.addEventListener("click", function () { window.inker.minimize(); });
if (maxBtn) maxBtn.addEventListener("click", function () { window.inker.maximize(); });
if (closeBtn) closeBtn.addEventListener("click", function () { window.inker.close(); });

// ====== Log Clear ======
if (clearLogBtn) {
    clearLogBtn.addEventListener("click", function () {
        if (eventLog) eventLog.innerHTML = "";
    });
}

// ====== App Initialization ======
// We ONLY use onReady — no fallback getSession() race condition
if (window.inker) {
    window.inker.onLog(function (message) {
        appendLog(message);
    });

    window.inker.onReady(async function (data) {
        appendLog("[App] onReady: valid=" + (data && data.valid ? "yes" : "no"));
        if (data && data.valid) {
            showMainScreen(data);
        } else {
            showLoginScreen();
        }
    });

    // Fallback: check session directly as a safety net
    (async function () {
        try {
            var session = await window.inker.getSession();
            if (session) {
                sessionCheckDone = true;
                showMainScreen(session);
                appendLog("Session restored for " + session.username);
            }
        } catch (e) {
            // onReady will handle it
        }
    })();

    // Backup timeout: if nothing has happened within 30s, show login
    setTimeout(function () {
        if (!sessionCheckDone) {
            appendLog("[App] Session check timeout — showing login");
            showLoginScreen();
        }
    }, 30000);
}
