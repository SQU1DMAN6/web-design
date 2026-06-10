const authScreen = document.getElementById("auth-screen");
const mainScreen = document.getElementById("main-screen");
const dropsScreen = document.getElementById("drops-screen");
const searchScreen = document.getElementById("search-screen");
const settingsScreen = document.getElementById("settings-screen");

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
    if (log.children.length > 200) {
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
    appendLog("Showing login screen");
    if (authScreen) authScreen.classList.add("active");
    if (mainScreen) mainScreen.classList.remove("active");
    if (emailInput) emailInput.focus();
}

function showMainScreen(user) {
    appendLog("Showing main screen for " + (user.username || user.email));
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
        const email = emailInput.value.trim();
        const password = passwordInput.value;

        if (authError) {
            authError.style.display = "none";
            authError.textContent = "";
        }

        try {
            appendLog("Attempting login for " + email, "auth");
            const result = await window.inker.login(email, password);
            appendLog("Login successful: " + result.username, "auth");
            loginForm.reset();
            showMainScreen(result);
        } catch (err) {
            appendLog("Login failed: " + err.message, "auth");
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
            if (screen === "drops" && dropsScreen) {
                dropsScreen.classList.add("active");
                appendLog("Navigated to Drops screen");
            }
            if (screen === "search" && searchScreen) {
                searchScreen.classList.add("active");
                appendLog("Navigated to Search screen");
            }
            if (screen === "settings" && settingsScreen) {
                settingsScreen.classList.add("active");
                appendLog("Navigated to Settings screen");
            }
        });
    });
}

if (logoutBtn) {
    logoutBtn.addEventListener("click", async function () {
        try {
            appendLog("Logging out...");
            await window.inker.logout();
            appendLog("Logout successful");
            await new Promise(function (resolve) { setTimeout(resolve, 500); });
            showLoginScreen();
        } catch (err) {
            appendLog("Logout failed: " + err.message);
        }
    });
}

const dropsList = document.getElementById("drops-list");
const noDropsMsg = document.getElementById("no-drops-message");

async function refreshDropsList() {
    try {
        appendLog("Refreshing Drops list...");
        const mounts = await window.inker.getActiveMounts();
        const saved = await window.inker.getSavedMounts();

        if (dropsList) dropsList.innerHTML = "";

        if (mounts.length === 0) {
            if (noDropsMsg) noDropsMsg.style.display = "block";
            appendLog("No active Drops found");
        } else {
            if (noDropsMsg) noDropsMsg.style.display = "none";

            mounts.forEach(function (mount) {
                const parts = mount.dropPath.split("/");
                const user = parts[0];
                const drop = parts[1];
                if (dropsList) {
                    const card = document.createElement("div");
                    card.className = "drop-card";
                    card.innerHTML =
                        '<div class="drop-info">' +
                            '<h3>' + user + '/' + drop + '</h3>' +
                            '<p class="mount-path">' + mount.mountPoint + '</p>' +
                        '</div>' +
                        '<span class="status status-mounted">Mounted</span>' +
                        '<button class="menu-btn" data-user="' + user + '" data-drop="' + drop + '">&#x22EE;</button>';
                    dropsList.appendChild(card);
                }
            });
            appendLog("Displaying " + mounts.length + " mounted Drop(s)");
        }

        refreshSettingsList(saved);
    } catch (err) {
        appendLog("Failed to refresh Drops: " + err.message);
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
            contextMenu.style.left = Math.max(10, rect.left) + "px";
            contextMenu.style.top = Math.max(10, rect.bottom + 6) + "px";
        }
        appendLog("Opened context menu for " + activeContextMenu.user + "/" + activeContextMenu.drop);
    } else if (!contextMenu || !contextMenu.contains(e.target)) {
        if (contextMenu) contextMenu.classList.add("hidden");
    }
});

const menuMount = document.getElementById("menu-mount");
if (menuMount) {
    menuMount.addEventListener("click", function () {
        if (!activeContextMenu) return;
        const dialog = document.getElementById("mount-dialog");
        if (dialog) {
            document.getElementById("mount-drop-name").value =
                activeContextMenu.user + "/" + activeContextMenu.drop;
            document.getElementById("mount-point").value =
                "C:\\Users\\" + (process.env.USERNAME || "user") + "\\FtR\\" +
                activeContextMenu.user + "\\" + activeContextMenu.drop;
            dialog.classList.remove("hidden");
            if (contextMenu) contextMenu.classList.add("hidden");
            appendLog("Mount dialog opened for " + activeContextMenu.user + "/" + activeContextMenu.drop);
        }
    });
}

const cancelMount = document.getElementById("cancel-mount-btn");
if (cancelMount) {
    cancelMount.addEventListener("click", function () {
        const dialog = document.getElementById("mount-dialog");
        if (dialog) dialog.classList.add("hidden");
        appendLog("Mount dialog cancelled");
    });
}

const doMount = document.getElementById("mount-btn");
if (doMount) {
    doMount.addEventListener("click", async function () {
        if (!activeContextMenu) return;
        const mountPoint = document.getElementById("mount-point").value;
        const mountError = document.getElementById("mount-error");
        if (mountError) mountError.style.display = "none";

        try {
            appendLog("Mounting " + activeContextMenu.user + "/" + activeContextMenu.drop + " at " + mountPoint);
            await window.inker.mountDrop(activeContextMenu.user, activeContextMenu.drop, mountPoint);
            appendLog("Mount successful: " + activeContextMenu.user + "/" + activeContextMenu.drop);
            const dialog = document.getElementById("mount-dialog");
            if (dialog) dialog.classList.add("hidden");
            await new Promise(function (r) { setTimeout(r, 500); });
            refreshDropsList();
        } catch (err) {
            appendLog("Mount failed: " + err.message);
            if (mountError) {
                mountError.textContent = err.message;
                mountError.style.display = "block";
            }
        }
    });
}

const doUnmount = document.getElementById("menu-unmount");
if (doUnmount) {
    doUnmount.addEventListener("click", async function () {
        if (!activeContextMenu) return;
        try {
            appendLog("Unmounting " + activeContextMenu.user + "/" + activeContextMenu.drop);
            await window.inker.unmountDrop(activeContextMenu.user, activeContextMenu.drop);
            appendLog("Unmount successful: " + activeContextMenu.user + "/" + activeContextMenu.drop);
            if (contextMenu) contextMenu.classList.add("hidden");
            refreshDropsList();
        } catch (err) {
            appendLog("Unmount failed: " + err.message);
        }
    });
}

const doOpen = document.getElementById("menu-open");
if (doOpen) {
    doOpen.addEventListener("click", async function () {
        if (!activeContextMenu) return;
        try {
            const mounts = await window.inker.getActiveMounts();
            const mount = mounts.find(function (m) {
                return m.dropPath === activeContextMenu.user + "/" + activeContextMenu.drop;
            });
            if (mount) {
                await window.inker.openPath(mount.mountPoint);
                appendLog("Opening folder: " + mount.mountPoint);
            }
            if (contextMenu) contextMenu.classList.add("hidden");
        } catch (err) {
            appendLog("Failed to open folder: " + err.message);
        }
    });
}

const autoMountBtn = document.getElementById("menu-auto-mount");
if (autoMountBtn) {
    autoMountBtn.addEventListener("click", async function () {
        if (!activeContextMenu) return;
        try {
            await window.inker.setAutoMount(activeContextMenu.user, activeContextMenu.drop, true);
            appendLog("Auto-mount enabled for " + activeContextMenu.user + "/" + activeContextMenu.drop);
            if (contextMenu) contextMenu.classList.add("hidden");
            const saved = await window.inker.getSavedMounts();
            refreshSettingsList(saved);
        } catch (err) {
            appendLog("Failed to set auto-mount: " + err.message);
        }
    });
}

const searchInput = document.getElementById("search-input");
const searchBtn = document.getElementById("search-btn");
const searchResults = document.getElementById("search-results");
const noSearchMsg = document.getElementById("no-search-message");

if (searchBtn && searchInput) {
    searchBtn.addEventListener("click", async function () {
        const query = searchInput.value.trim();
        if (!query) return;

        try {
            appendLog("Searching Drops for: " + query);
            const results = await window.inker.searchDrops(query);
            appendLog("Search returned " + results.length + " result(s)");

            if (searchResults) searchResults.innerHTML = "";
            if (noSearchMsg) noSearchMsg.style.display = results.length === 0 ? "block" : "none";

            results.forEach(function (drop) {
                if (searchResults) {
                    const card = document.createElement("div");
                    card.className = "search-result-card";
                    card.innerHTML =
                        '<h4>' + (drop.user || "user") + '/' + (drop.name || "drop") + '</h4>' +
                        '<p>' + (drop.description || "No description") + '</p>' +
                        '<button class="btn btn-small" data-user="' + drop.user + '" data-drop="' + drop.name + '">Mount</button>';
                    searchResults.appendChild(card);
                }
            });
        } catch (err) {
            appendLog("Search failed: " + err.message);
        }
    });

    searchInput.addEventListener("keypress", function (e) {
        if (e.key === "Enter") searchBtn.click();
    });
}

document.addEventListener("click", function (e) {
    if (e.target.closest && e.target.closest(".search-result-card")) {
        const btn = e.target.closest("button");
        if (btn && btn.dataset.user) {
            activeContextMenu = { user: btn.dataset.user, drop: btn.dataset.drop };
            if (menuMount) menuMount.click();
        }
    }
});

function refreshSettingsList(mounts) {
    const autoMountList = document.getElementById("auto-mount-list");
    if (!autoMountList) return;

    autoMountList.innerHTML = "";

    const autoMounts = mounts.filter(function (m) { return m.auto_mount; });
    if (autoMounts.length === 0) {
        autoMountList.innerHTML = '<p class="empty-state">No auto-mount Drops configured.</p>';
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
                refreshSettingsList(saved);
                appendLog("Auto-mount disabled for " + dropPath);
            } catch (err) {
                appendLog("Failed to disable auto-mount: " + err.message);
            }
        });
    });
}

const mountByPathInput = document.getElementById("mount-by-path-input");
const mountByPathBtn = document.getElementById("mount-by-path-btn");

if (mountByPathBtn && mountByPathInput) {
    mountByPathBtn.addEventListener("click", async function () {
        var path = mountByPathInput.value.trim();
        if (!path) return;
        var parts = path.split("/");
        if (parts.length !== 2 || !parts[0] || !parts[1]) {
            appendLog("Invalid Drop path: " + path + " (must be user/repo)");
            return;
        }
        var user = parts[0];
        var drop = parts[1];
        try {
            appendLog("Verifying Drop " + user + "/" + drop + "...");
            var exists = await window.inker.verifyDrop(user, drop);
            if (exists) {
                appendLog("Drop " + user + "/" + drop + " exists, opening mount dialog");
                activeContextMenu = { user: user, drop: drop };
                if (menuMount) menuMount.click();
            } else {
                appendLog("Drop " + user + "/" + drop + " does not exist or is not accessible");
            }
        } catch (err) {
            appendLog("Failed to verify Drop: " + err.message);
        }
    });

    mountByPathInput.addEventListener("keypress", function (e) {
        if (e.key === "Enter") mountByPathBtn.click();
    });
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
        appendLog("Application ready");
        const user = await window.inker.getCurrentUser();
        if (user) {
            showMainScreen(user);
            appendLog("Session restored for " + user.username);
        } else {
            showLoginScreen();
        }
    });
}

if (window.inker) {
    window.inker.getCurrentUser().then(function (user) {
        if (user) {
            showMainScreen(user);
            appendLog("Session loaded for " + user.username);
        } else {
            showLoginScreen();
        }
    }).catch(function () {
        showLoginScreen();
    });
}