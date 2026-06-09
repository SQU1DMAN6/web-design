// Production-grade Inker renderer with full functionality

// Screen Navigation
const authScreen = document.getElementById("auth-screen");
const mainScreen = document.getElementById("main-screen");
const reposScreen = document.getElementById("repos-screen");
const searchScreen = document.getElementById("search-screen");
const settingsScreen = document.getElementById("settings-screen");

const navBtns = document.querySelectorAll(".nav-btn");
const logoutBtn = document.getElementById("logout-btn");

// Event Logging
const eventLog = document.getElementById("event-log");
const eventLogAuth = document.getElementById("event-log-auth");
const clearLogBtn = document.getElementById("clear-log-btn");

function appendLog(message, screen = "main") {
    const log = screen === "auth" ? eventLogAuth : eventLog;
    if (!log) return;
    const entry = document.createElement("div");
    entry.className = "log-entry";
    entry.textContent = `${new Date().toLocaleTimeString()} — ${message}`;
    log.prepend(entry);
    if (log.children.length > 100) {
        log.removeChild(log.lastChild);
    }
    console.log(`[Inker] ${message}`);
}

if (clearLogBtn) {
    clearLogBtn.addEventListener("click", () => {
        eventLog.innerHTML = "";
        appendLog("Log cleared");
    });
}

// Auth Screen Functions
const loginForm = document.getElementById("login-form");
const emailInput = document.getElementById("email");
const passwordInput = document.getElementById("password");
const authError = document.getElementById("auth-error");

async function showLoginScreen() {
    if (authScreen) authScreen.classList.add("active");
    if (mainScreen) mainScreen.classList.remove("active");
    if (emailInput) emailInput.focus();
}

async function showMainScreen(user) {
    if (authScreen) authScreen.classList.remove("active");
    if (mainScreen) mainScreen.classList.add("active");
    if (document.getElementById("sidebar-username")) document.getElementById("sidebar-username").textContent = user.username || user.email;
    if (document.getElementById("sidebar-email")) document.getElementById("sidebar-email").textContent = user.email;
    refreshReposList();
}

if (loginForm) {
    loginForm.addEventListener("submit", async (e) => {
        e.preventDefault();
        const email = emailInput.value.trim();
        const password = passwordInput.value;

        if (authError) {
            authError.style.display = "none";
            authError.textContent = "";
        }

        try {
            appendLog(`Attempting login for ${email}`, "auth");
            const result = await window.inker.login(email, password);
            appendLog(`✓ Logged in as ${result.username}`, "auth");
            loginForm.reset();
            showMainScreen(result);
        } catch (err) {
            appendLog(`✗ Login failed: ${err.message}`, "auth");
            if (authError) {
                authError.textContent = err.message || "Login failed";
                authError.style.display = "block";
            }
        }
    });
}

// Screen Navigation
if (navBtns.length > 0) {
    navBtns.forEach(btn => {
        btn.addEventListener("click", () => {
            navBtns.forEach(b => b.classList.remove("active"));
            btn.classList.add("active");

            const screen = btn.dataset.screen;
            document.querySelectorAll(".main-screen").forEach(s => s.classList.remove("active"));

            if (screen === "repos" && reposScreen) reposScreen.classList.add("active");
            if (screen === "search" && searchScreen) searchScreen.classList.add("active");
            if (screen === "settings" && settingsScreen) settingsScreen.classList.add("active");
        });
    });
}

// Logout
if (logoutBtn) {
    logoutBtn.addEventListener("click", async () => {
        try {
            await window.inker.logout();
            appendLog("✓ Logged out");
            await new Promise(resolve => setTimeout(resolve, 500));
            showLoginScreen();
        } catch (err) {
            appendLog(`✗ Logout failed: ${err.message}`);
        }
    });
}

// Repos Management
const mountsList = document.getElementById("mounts-list");
const noReposMsg = document.getElementById("no-repos-message");

async function refreshReposList() {
    try {
        const mounts = await window.inker.getActiveMounts();
        const saved = await window.inker.getSavedMounts();

        if (mountsList) mountsList.innerHTML = "";

        if (mounts.length === 0) {
            if (noReposMsg) noReposMsg.style.display = "block";
        } else {
            if (noReposMsg) noReposMsg.style.display = "none";

            mounts.forEach(mount => {
                const [user, repo] = mount.repoPath.split("/");
                if (mountsList) {
                    const card = document.createElement("div");
                    card.className = "repo-card";
                    card.innerHTML = `
                        <div class="repo-info">
                            <h3>${user}/${repo}</h3>
                            <p class="mount-path">${mount.mountPoint}</p>
                        </div>
                        <span class="status online">Mounted</span>
                        <button class="menu-btn" data-user="${user}" data-repo="${repo}">⋮</button>
                    `;
                    mountsList.appendChild(card);
                }
            });
        }

        refreshSettingsList(saved);
    } catch (err) {
        appendLog(`✗ Failed to refresh repos: ${err.message}`);
    }
}

// Context Menu
const contextMenu = document.getElementById("repo-menu");
let activeContextMenu = null;

document.addEventListener("click", (e) => {
    if (e.target.classList.contains("menu-btn")) {
        e.stopPropagation();
        activeContextMenu = {
            user: e.target.dataset.user,
            repo: e.target.dataset.repo
        };

        const rect = e.target.getBoundingClientRect();
        if (contextMenu) {
            contextMenu.classList.remove("hidden");
            contextMenu.style.left = `${Math.max(10, rect.left)}px`;
            contextMenu.style.top = `${Math.max(10, rect.bottom + 6)}px`;
        }
    } else if (!contextMenu || !contextMenu.contains(e.target)) {
        if (contextMenu) contextMenu.classList.add("hidden");
    }
});

const menuMount = document.getElementById("menu-mount");
if (menuMount) {
    menuMount.addEventListener("click", async () => {
        if (!activeContextMenu) return;
        const dialog = document.getElementById("mount-dialog");
        if (dialog) {
            document.getElementById("mount-repo-name").value = `${activeContextMenu.user}/${activeContextMenu.repo}`;
            document.getElementById("mount-point").value = `C:\\Users\\${process.env.USERNAME || 'user'}\\FtR\\${activeContextMenu.user}\\${activeContextMenu.repo}`;
            dialog.classList.remove("hidden");
            if (contextMenu) contextMenu.classList.add("hidden");
        }
    });
}

const cancelMount = document.getElementById("cancel-mount-btn");
if (cancelMount) {
    cancelMount.addEventListener("click", () => {
        const dialog = document.getElementById("mount-dialog");
        if (dialog) dialog.classList.add("hidden");
    });
}

const doMount = document.getElementById("mount-btn");
if (doMount) {
    doMount.addEventListener("click", async () => {
        if (!activeContextMenu) return;
        const mountPoint = document.getElementById("mount-point").value;
        const mountError = document.getElementById("mount-error");
        if (mountError) mountError.style.display = "none";

        try {
            appendLog(`Mounting ${activeContextMenu.user}/${activeContextMenu.repo} at ${mountPoint}`);
            await window.inker.mountRepository(activeContextMenu.user, activeContextMenu.repo, mountPoint);
            appendLog(`✓ Mounted ${activeContextMenu.user}/${activeContextMenu.repo}`);
            const dialog = document.getElementById("mount-dialog");
            if (dialog) dialog.classList.add("hidden");
            await new Promise(r => setTimeout(r, 500));
            refreshReposList();
        } catch (err) {
            appendLog(`✗ Mount failed: ${err.message}`);
            if (mountError) {
                mountError.textContent = err.message;
                mountError.style.display = "block";
            }
        }
    });
}

const doUnmount = document.getElementById("menu-unmount");
if (doUnmount) {
    doUnmount.addEventListener("click", async () => {
        if (!activeContextMenu) return;
        try {
            appendLog(`Unmounting ${activeContextMenu.user}/${activeContextMenu.repo}`);
            await window.inker.unmountRepository(activeContextMenu.user, activeContextMenu.repo);
            appendLog(`✓ Unmounted ${activeContextMenu.user}/${activeContextMenu.repo}`);
            if (contextMenu) contextMenu.classList.add("hidden");
            refreshReposList();
        } catch (err) {
            appendLog(`✗ Unmount failed: ${err.message}`);
        }
    });
}

const doOpen = document.getElementById("menu-open");
if (doOpen) {
    doOpen.addEventListener("click", async () => {
        if (!activeContextMenu) return;
        try {
            const mounts = await window.inker.getActiveMounts();
            const mount = mounts.find(m => m.repoPath === `${activeContextMenu.user}/${activeContextMenu.repo}`);
            if (mount) {
                await window.inker.openPath(mount.mountPoint);
                appendLog(`Opening ${mount.mountPoint}`);
            }
            if (contextMenu) contextMenu.classList.add("hidden");
        } catch (err) {
            appendLog(`✗ Failed to open folder: ${err.message}`);
        }
    });
}

const autoMountBtn = document.getElementById("menu-auto-mount");
if (autoMountBtn) {
    autoMountBtn.addEventListener("click", async () => {
        if (!activeContextMenu) return;
        try {
            await window.inker.setAutoMount(activeContextMenu.user, activeContextMenu.repo, true);
            appendLog(`✓ Auto-mount enabled for ${activeContextMenu.user}/${activeContextMenu.repo}`);
            if (contextMenu) contextMenu.classList.add("hidden");
            const saved = await window.inker.getSavedMounts();
            refreshSettingsList(saved);
        } catch (err) {
            appendLog(`✗ Failed to set auto-mount: ${err.message}`);
        }
    });
}

// Search
const searchInput = document.getElementById("search-input");
const searchBtn = document.getElementById("search-btn");
const searchResults = document.getElementById("search-results");
const noSearchMsg = document.getElementById("no-search-message");

if (searchBtn && searchInput) {
    searchBtn.addEventListener("click", async () => {
        const query = searchInput.value.trim();
        if (!query) return;

        try {
            appendLog(`Searching for "${query}"`);
            const results = await window.inker.searchRepositories(query);
            appendLog(`Found ${results.length} repositories`);

            if (searchResults) searchResults.innerHTML = "";
            if (noSearchMsg) noSearchMsg.style.display = results.length === 0 ? "block" : "none";

            results.forEach(repo => {
                if (searchResults) {
                    const card = document.createElement("div");
                    card.className = "search-result-card";
                    card.innerHTML = `
                        <h4>${repo.user || "user"}/${repo.name || "repo"}</h4>
                        <p>${repo.description || "No description"}</p>
                        <button class="btn btn-small" data-user="${repo.user}" data-repo="${repo.name}">Mount</button>
                    `;
                    searchResults.appendChild(card);
                }
            });

            document.querySelectorAll(".search-result-card button").forEach(btn => {
                btn.addEventListener("click", () => {
                    activeContextMenu = { user: btn.dataset.user, repo: btn.dataset.repo };
                    if (menuMount) menuMount.click();
                });
            });
        } catch (err) {
            appendLog(`✗ Search failed: ${err.message}`);
        }
    });

    searchInput.addEventListener("keypress", (e) => {
        if (e.key === "Enter") searchBtn.click();
    });
}

// Settings - Auto-mount
function refreshSettingsList(mounts) {
    const autoMountList = document.getElementById("auto-mount-list");
    if (!autoMountList) return;

    autoMountList.innerHTML = "";

    const autoMounts = mounts.filter(m => m.auto_mount);
    if (autoMounts.length === 0) {
        autoMountList.innerHTML = "<p class='empty'>No auto-mount repositories configured.</p>";
        return;
    }

    autoMounts.forEach(mount => {
        const item = document.createElement("div");
        item.className = "auto-mount-item";
        item.innerHTML = `
            <span>${mount.repo_path} → ${mount.mount_point}</span>
            <button class="btn btn-small" data-repo-path="${mount.repo_path}">Disable</button>
        `;
        autoMountList.appendChild(item);
    });

    document.querySelectorAll(".auto-mount-item button").forEach(btn => {
        btn.addEventListener("click", async () => {
            const repoPath = btn.dataset.repoPath;
            const [user, repo] = repoPath.split("/");
            try {
                await window.inker.setAutoMount(user, repo, false);
                const saved = await window.inker.getSavedMounts();
                refreshSettingsList(saved);
                appendLog(`✓ Auto-mount disabled for ${repoPath}`);
            } catch (err) {
                appendLog(`✗ Failed to disable auto-mount: ${err.message}`);
            }
        });
    });
}

// Window Controls
const minBtn = document.getElementById("min-btn");
if (minBtn) minBtn.addEventListener("click", () => window.inker.minimize());

const maxBtn = document.getElementById("max-btn");
if (maxBtn) maxBtn.addEventListener("click", () => window.inker.maximize());

const closeBtn = document.getElementById("close-btn");
if (closeBtn) closeBtn.addEventListener("click", () => window.inker.close());

// Logging
if (window.inker) {
    window.inker.onLog((message) => {
        appendLog(message);
    });

    window.inker.onReady(async () => {
        const user = await window.inker.getCurrentUser();
        if (user) {
            showMainScreen(user);
            appendLog(`Session restored for ${user.username}`);
        } else {
            showLoginScreen();
        }
    });
}

// Initial check
if (window.inker) {
    window.inker.getCurrentUser().then(user => {
        if (user) {
            showMainScreen(user);
            appendLog(`Session loaded for ${user.username}`);
        } else {
            showLoginScreen();
        }
    }).catch(() => {
        showLoginScreen();
    });
}
