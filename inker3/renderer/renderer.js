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
const searchScreen = document.getElementById("search-screen");
const logScreen = document.getElementById("log-screen");

// ====== Drops Panel ======
const dropsList = document.getElementById("drops-list");
const noDropsMsg = document.getElementById("no-drops-message");

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

// ====== State ======
var authData = null;
var sessionCheckDone = false;

// ====== Log Helper ======
function appendLog(message) {
    if (!eventLog) return;
    var entry = document.createElement("div");
    entry.className = "log-entry";
    var ts = new Date().toISOString().replace("T", " ").slice(0, 19);
    entry.textContent = ts + " " + message;
    eventLog.prepend(entry);
    if (eventLog.children.length > 500) {
        eventLog.removeChild(eventLog.lastChild);
    }
    console.log("[Inker] " + message);
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

// Build HTML-safe replacement strings (using hex escapes to avoid auto-formatting issues)
var _AMP = String.fromCharCode(38) + "amp;";
var _LT = String.fromCharCode(38) + "lt;";
var _GT = String.fromCharCode(38) + "gt;";
var _QUOT = String.fromCharCode(38) + "quot;";

function escapeHtml(str) {
    if (!str) return "";
    return String(str)
        .replace(/&/g, _AMP)
        .replace(/</g, _LT)
        .replace(/>/g, _GT)
        .replace(/"/g, _QUOT);
}

// ====== Drops List ======
async function refreshDropsList() {
    try {
        var mounts = await window.inker.getActiveMounts();
        if (dropsList) dropsList.innerHTML = "";
        if (noDropsMsg) noDropsMsg.style.display = mounts.length === 0 ? "block" : "none";

        for (var i = 0; i < mounts.length; i++) {
            var mount = mounts[i];
            var parts = mount.dropKey ? mount.dropKey.split("/") : [];
            var user = parts[0] || mount.user;
            var drop = parts[1] || mount.drop;

            var safeUser = escapeHtml(user);
            var safeDrop = escapeHtml(drop);
            var safeMountPath = escapeHtml(mount.mountPath || "Q:\\");

            var section = document.createElement("div");
            section.className = "mount-section";

            var card = document.createElement("div");
            card.className = "drop-card";
            card.innerHTML =
                '<div class="drop-info">' +
                    '<h3>' + safeUser + '/' + safeDrop + '</h3>' +
                    '<p class="mount-path">' + safeMountPath + '</p>' +
                '</div>' +
                '<div class="drop-actions">' +
                    '<button class="btn btn-small open-folder-btn" data-path="' + safeMountPath + '">Open</button>' +
                    '<button class="btn btn-small btn-danger unmount-btn" data-user="' + safeUser + '" data-drop="' + safeDrop + '">Unmount</button>' +
                '</div>';
            section.appendChild(card);
            dropsList.appendChild(section);
        }

        document.querySelectorAll(".unmount-btn").forEach(function (btn) {
            btn.addEventListener("click", async function () {
                var user = btn.dataset.user;
                var drop = btn.dataset.drop;
                appendLog("Unmounting " + user + "/" + drop + "...");
                try {
                    await window.inker.unmountDrop(user, drop);
                    appendLog("Unmounted " + user + "/" + drop);
                    refreshDropsList();
                } catch (err) { appendLog("Unmount error: " + err.message); }
            });
        });
    } catch (err) {
        appendLog("refreshDropsList error: " + err.message);
    }
}

// Open folder - uses the actual mount path from the button's data-path attribute
document.addEventListener("click", function (e) {
    var openBtn = e.target.closest(".open-folder-btn");
    if (openBtn) {
        e.stopPropagation();
        var mountPath = openBtn.dataset.path;
        if (mountPath) {
            window.inker.openPath(mountPath);
        }
        return;
    }
});

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
                var safeUser = escapeHtml(drop.user);
                var safeName = escapeHtml(drop.name);
                var safeDesc = escapeHtml(drop.description || "No description");

                var item = document.createElement("div");
                item.className = "drop-card search-result-card";
                item.innerHTML =
                    '<div class="drop-info">' +
                        '<h3>' + safeUser + '/' + safeName + '</h3>' +
                        '<p class="drop-desc">' + safeDesc + '</p>' +
                    '</div>' +
                    '<div class="drop-actions">' +
                        '<button class="btn btn-primary btn-small mount-from-search" data-user="' + safeUser + '" data-drop="' + safeName + '">Mount</button>' +
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

// Mount from search results
document.addEventListener("click", function (e) {
    var mountBtn = e.target.closest(".mount-from-search");
    if (mountBtn) {
        mountBtn.textContent = "Mounting...";
        mountBtn.disabled = true;
        mountDrop(mountBtn.dataset.user, mountBtn.dataset.drop, mountBtn);
    }
});

// ====== Mount Drop ======
async function mountDrop(user, drop, button) {
    appendLog("Mounting " + user + "/" + drop + "...");
    try {
        await window.inker.mountDrop(user, drop);
        appendLog("Mounted " + user + "/" + drop + " on Q:\\");
        if (button) {
            button.textContent = "Mounted";
            button.className = "btn btn-small btn-added";
            button.disabled = false;
        }
        refreshDropsList();
    } catch (err) {
        appendLog("Mount error: " + err.message);
        if (button) { button.textContent = "Mount"; button.disabled = false; }
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
        appendLog("Mounting by path: " + parts[0] + "/" + parts[1]);
        addByPathBtn.textContent = "Mounting...";
        addByPathBtn.disabled = true;
        await mountDrop(parts[0], parts[1], addByPathBtn);
        addByPathBtn.textContent = "Mount";
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

    (async function () {
        try {
            var session = await window.inker.getSession();
            if (session) {
                sessionCheckDone = true;
                showMainScreen(session);
                appendLog("Session restored for " + session.username);
            }
        } catch (e) {}
    })();

    setTimeout(function () {
        if (!sessionCheckDone) {
            appendLog("[App] Session check timeout --- showing login");
            showLoginScreen();
        }
    }, 30000);
}