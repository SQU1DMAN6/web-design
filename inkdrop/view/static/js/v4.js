/**
 * InkDrop 4.0 — Client-Side Application
 * Full file manager with state management, context menus, drag-and-drop,
 * multi-selection, and permission-aware UI.
 */
(function () {
    'use strict';

    var CONFIG = window.__INKDROP_INIT__ || {
        userName: 'Guest',
        userID: 0,
        userPFP: '/pfp/default.png',
        userBio: '',
        currentDropID: '',
        currentDropName: '',
        apiBase: '/api/v4'
    };

    // === Application State ===
    var state = {
        // UserState
        user: {
            name: CONFIG.userName,
            pfp: CONFIG.userPFP,
            bio: CONFIG.userBio
        },

        // DropState
        drop: {
            current: null,
            members: [],
            permissions: 'viewer'
        },

        // FileState
        file: {
            currentDir: '/',
            allFiles: [],
            currentFiles: [],
            selected: [],          // array of file IDs
            viewMode: 'grid',
            sortOrder: 'name',
            searchQuery: '',
            loading: false,
            error: null,
            trash: []
        },

        // UIState
        ui: {
            activeTab: 'home',
            theme: 'light',
            contextMenu: null,
            dragSource: null,
            dialogs: {}
        },

        // Legacy
        drops: [],
        allDrops: [],
        sharedDrops: [],
        publicDrops: [],
        contacts: [],
        searchResults: null
    };

    // === DOM Shortcuts ===
    var $ = function (id) { return document.getElementById(id); };
    var qs = function (s, r) { return (r || document).querySelector(s); };
    var qsa = function (s, r) { return Array.prototype.slice.call((r || document).querySelectorAll(s)); };

    // === API Client ===
    var api = {
        base: CONFIG.apiBase,
        get: function (path) {
            return fetch(this.base + path, { credentials: 'same-origin' }).then(function (r) { return r.json(); });
        },
        post: function (path, body) {
            return fetch(this.base + path, {
                method: 'POST', credentials: 'same-origin',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body)
            }).then(function (r) { return r.json(); });
        },
        put: function (path, body) {
            return fetch(this.base + path, {
                method: 'PUT', credentials: 'same-origin',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body)
            }).then(function (r) { return r.json(); });
        },
        del: function (path) {
            return fetch(this.base + path, { method: 'DELETE', credentials: 'same-origin' }).then(function (r) { return r.json(); });
        },
        upload: function (path, formData) {
            return fetch(this.base + path, {
                method: 'POST', credentials: 'same-origin',
                body: formData
            }).then(function (r) { return r.json(); });
        },

        // Drops
        listDrops: function () { return this.get('/drops'); },
        getDrop: function (id) { return this.get('/drop/' + id); },
        createDrop: function (name, desc) { return this.post('/drops', { name: name, description: desc || '' }); },
        deleteDrop: function (id) { return this.del('/drop/' + id); },

        // Files
        listFiles: function (id, path) {
            var q = path && path !== '/' ? '?path=' + encodeURIComponent(path) : '';
            return this.get('/drop/' + id + '/files' + q);
        },
        createFolder: function (id, name, parent) {
            return this.post('/drop/' + id + '/folders', { name: name, parent: parent || '/' });
        },
        uploadFile: function (id, file, path) {
            var fd = new FormData();
            fd.append('file', file);
            fd.append('path', path || '/');
            return this.upload('/drop/' + id + '/upload', fd);
        },
        getFileContent: function (did, fid) { return this.get('/drop/' + did + '/files/' + fid + '/content'); },
        renameFile: function (did, fid, name) {
            return this.post('/drop/' + did + '/files/' + fid + '/rename', { name: name });
        },
        moveFile: function (did, fid, dest) {
            return this.post('/drop/' + did + '/files/' + fid + '/move', { destination: dest });
        },
        trashFile: function (did, fid) {
            return this.post('/drop/' + did + '/files/' + fid + '/trash', {});
        },
        restoreFile: function (did, fid) {
            return this.post('/drop/' + did + '/files/' + fid + '/restore', {});
        },
        deleteFile: function (did, fid) {
            return this.del('/drop/' + did + '/files/' + fid);
        },
        listTrash: function (id) { return this.get('/drop/' + id + '/trash'); },
        emptyTrash: function (id) { return this.post('/drop/' + id + '/trash/empty', {}); },

        // Sharing
        shareDrop: function (id, userName, perm) {
            return this.post('/drop/' + id + '/share', { user_name: userName, permission: perm || 'viewer' });
        },
        getMembers: function (id) { return this.get('/drop/' + id + '/members'); },
        updateMemberRole: function (did, uid, role) {
            return this.put('/drop/' + did + '/members/' + uid + '/role', { role: role });
        },
        removeMember: function (did, uid) {
            return this.del('/drop/' + did + '/members/' + uid);
        },
        updateDrop: function (id, data) {
            return this.put('/drop/' + id, data);
        },
        deleteDrop: function (id) {
            return this.del('/drop/' + id);
        },

        // Activity
        getActivity: function (id) { return this.get('/drop/' + id + '/activity'); },

        // Search
        search: function (q) { return this.get('/search?q=' + encodeURIComponent(q)); },

        // Contacts
        getContacts: function () { return this.get('/contacts'); },
        addContact: function (username) { return this.post('/contacts', { username: username }); },
        removeContact: function (name) { return this.del('/contacts/' + encodeURIComponent(name)); },
        searchUsers: function (q) { return this.get('/contacts/search?q=' + encodeURIComponent(q)); }
    };

    // === Toast Notification ===
    function showToast(message, type) {
        type = type || 'info';
        var container = $('toastContainer');
        if (!container) {
            container = document.createElement('div');
            container.id = 'toastContainer';
            container.className = 'v4-toast-container';
            document.body.appendChild(container);
        }
        var toast = document.createElement('div');
        toast.className = 'v4-toast v4-toast-' + type;
        toast.textContent = message;
        container.appendChild(toast);
        setTimeout(function () {
            toast.classList.add('v4-toast-hide');
            setTimeout(function () { toast.remove(); }, 300);
        }, 3000);
    }

    // === Error Handling ===
    function handleApiError(resp, fallback) {
        if (resp && resp.error && resp.error.message) {
            return resp.error.message;
        }
        return fallback || 'An error occurred';
    }

    // === Theme ===
    function initTheme() {
        var key = 'inkdrop-theme';
        function setTheme(v) {
            document.documentElement.setAttribute('data-theme', v);
            state.ui.theme = v;
            var ts = qs('.theme-switch');
            if (ts) ts.classList.toggle('is-dark', v === 'dark');
            qsa('[data-theme-set]').forEach(function (b) {
                b.setAttribute('aria-pressed', String(b.getAttribute('data-theme-set') === v));
            });
        }
        var stored = localStorage.getItem(key);
        var theme = (stored === 'dark' || stored === 'light') ? stored :
            (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
        setTheme(theme);
        qsa('[data-theme-set]').forEach(function (b) {
            b.type = 'button';
            b.addEventListener('click', function () {
                var t = b.getAttribute('data-theme-set');
                setTheme(t);
                localStorage.setItem(key, t);
            });
        });
    }

    // === Tab Navigation ===
    function switchTab(tab) {
        state.ui.activeTab = tab;
        qsa('.v4-nav-link[data-tab]').forEach(function (b) {
            b.classList.toggle('active', b.getAttribute('data-tab') === tab);
        });
        var bc = $('breadcrumb');
        if (bc) {
            var names = { home: 'My Drops', shared: 'Shared With Me', public: 'Public Drops', contacts: 'Contacts' };
            bc.innerHTML = '<span class="v4-breadcrumb-item active">' + (names[tab] || tab) + '</span>';
        }
        var controls = $('workspaceControls');
        if (controls) controls.style.display = (tab === 'home' || tab === 'drop') ? '' : 'none';

        var dropPanel = $('dropPanelContent');
        if (dropPanel && tab !== 'drop' && tab !== 'home') {
            dropPanel.innerHTML = '<div class="v4-drop-panel-placeholder"><svg viewBox="0 0 24 24" width="32" height="32" stroke="var(--muted)" stroke-width="1.5" fill="none"><path d="M6 3h9l4 4v14H6z"/><path d="M15 3v5h5"/></svg><p>Select a Drop<br/>to view details</p></div>';
        }

        switch (tab) {
            case 'home': renderDropBrowser(); break;
            case 'shared': renderSharedView(); break;
            case 'public': renderPublicView(); break;
            case 'contacts': renderContactsView(); break;
            default: renderDropBrowser();
        }
    }

    // === Main: Drop Browser (Home) ===
    function renderDropBrowser() {
        var content = $('workspaceContent');
        if (!content) return;
        content.innerHTML = '<div class="v4-workspace-empty"><p>Loading your drops...</p></div>';

        api.listDrops().then(function (resp) {
            if (!resp.success) {
                content.innerHTML = '<div class="v4-workspace-empty"><h3>Could not load drops</h3><p>' +
                    handleApiError(resp, 'Authentication error. Please try logging in again.') +
                    '</p><a href="/login" class="redirect" style="display:inline-block;margin-top:12px;text-decoration:none;">Log in</a></div>';
                return;
            }
            state.drops = resp.data || [];
            renderDropGrid(content);
        }).catch(function () {
            content.innerHTML = '<div class="v4-workspace-empty"><h3>Connection error</h3><p>Could not reach the server. Please try again.</p></div>';
        });
    }

    function renderDropGrid(container) {
        if (!state.drops || state.drops.length === 0) {
            container.innerHTML = '' +
                '<div class="v4-workspace-empty">' +
                '<svg viewBox="0 0 24 24" width="48" height="48" stroke="var(--muted)" stroke-width="1" fill="none"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/></svg>' +
                '<h3>Welcome to InkDrop!</h3>' +
                '<p>You don\'t have any Drops yet. Create one to get started.</p>' +
                '<button type="button" class="redirect" id="emptyCreateDrop">Create Your First Drop</button>' +
                '</div>';
            var btn = $('emptyCreateDrop');
            if (btn) btn.addEventListener('click', function () {
                var d = $('createDropDialog');
                if (d) d.showModal();
            });
            return;
        }

        var viewMode = state.file.viewMode;

        if (viewMode === 'grid') {
            var grid = document.createElement('div');
            grid.className = 'v4-drop-grid';
            state.drops.forEach(function (drop) {
                var card = document.createElement('div');
                card.className = 'v4-drop-card';
                card.innerHTML = '' +
                    '<h4 class="v4-drop-card-name">' + esc(drop.name) + '</h4>' +
                    '<p class="v4-drop-card-desc">' + (drop.description ? esc(drop.description) : 'No description') + '</p>' +
                    '<div class="v4-drop-card-meta">' +
                    '<span class="v4-badge v4-badge-' + (drop.visibility || 'private') + '">' + (drop.visibility || 'private') + '</span>' +
                    '<span>' + fmtTime(drop.updated_at) + '</span>' +
                    '</div>';
                card.addEventListener('click', function () { openDrop(drop); });
                grid.appendChild(card);
            });
            container.innerHTML = '';
            container.appendChild(grid);
        } else {
            var list = document.createElement('div');
            list.className = 'v4-file-list';
            state.drops.forEach(function (drop) {
                var item = document.createElement('div');
                item.className = 'v4-file-list-item';
                item.innerHTML = '' +
                    '<div class="v4-file-list-icon"><svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" fill="currentColor" opacity="0.3"/><path d="M20 6v12a2 2 0 01-2 2H6l-4-4V5a2 2 0 012-2h11l2 2h5a2 2 0 012 2v-1z" fill="currentColor"/></svg></div>' +
                    '<div class="v4-file-list-name" style="font-weight:800;">' + esc(drop.name) + '</div>' +
                    '<div class="v4-file-list-size">' + (drop.visibility || 'private') + '</div>' +
                    '<div class="v4-file-list-date">' + fmtTime(drop.updated_at) + '</div>';
                item.addEventListener('click', function () { openDrop(drop); });
                list.appendChild(item);
            });
            container.innerHTML = '';
            container.appendChild(list);
        }
    }

    // === Drop View ===
    function openDrop(drop) {
        state.drop.current = drop;
        state.ui.activeTab = 'drop';
        state.file.currentDir = '/';
        state.file.selected = [];

        // Update breadcrumb
        var bc = $('breadcrumb');
        if (bc) {
            bc.innerHTML = '<a href="/" class="v4-breadcrumb-item" id="bcHome">My Drops</a>' +
                '<span class="v4-breadcrumb-sep">/</span>' +
                '<span class="v4-breadcrumb-item active">' + esc(drop.name) + '</span>';
            var homeLink = $('bcHome');
            if (homeLink) homeLink.addEventListener('click', function (e) { e.preventDefault(); switchTab('home'); });
        }

        updateDropPanel(drop);
        loadFiles(drop.id, '/');
        loadMembers(drop.id);
    }

    function loadMembers(dropID) {
        api.getMembers(dropID).then(function (resp) {
            if (resp.success) {
                state.drop.members = resp.data || [];
                // Determine current user's role
                var me = state.drop.members.find(function (m) {
                    return m.user_id === getCurrentUserId();
                });
                if (me) {
                    state.drop.permissions = me.role;
                } else {
                    state.drop.permissions = 'viewer';
                }
                updatePermissionUI();
            }
        });
    }

    function getCurrentUserId() {
        return CONFIG.userID || 0;
    }

    function updatePermissionUI() {
        var canEdit = state.drop.permissions === 'editor' || state.drop.permissions === 'owner';
        var canManage = state.drop.permissions === 'owner';

        // Show/hide action buttons
        qsa('[data-action="upload"]').forEach(function (b) { b.style.display = canEdit ? '' : 'none'; });
        qsa('[data-action="new-folder"]').forEach(function (b) { b.style.display = canEdit ? '' : 'none'; });
        qsa('[data-action="delete-drop"]').forEach(function (b) { b.style.display = canManage ? '' : 'none'; });

        // Show/hide share button
        var shareBtn = $('shareDropBtn');
        if (shareBtn) shareBtn.style.display = canManage ? '' : 'none';
    }

    function loadFiles(dropID, path) {
        var content = $('workspaceContent');
        if (!content) return;
        state.file.loading = true;
        state.file.error = null;
        content.innerHTML = '<div class="v4-workspace-empty"><p>Loading files...</p></div>';

        // Load all files for the tree, then filter for the current directory view
        api.listFiles(dropID, '/').then(function (resp) {
            state.file.loading = false;
            if (resp.success) {
                state.file.allFiles = resp.data || [];
                state.file.currentDir = path || '/';
                state.file.selected = [];

                // Filter to files directly in the current directory
                var dir = state.file.currentDir;
                if (dir === '/' || dir === '') {
                    state.file.currentFiles = state.file.allFiles.filter(function (f) {
                        return f.path.indexOf('/') === -1;
                    });
                } else {
                    var prefix = dir.replace(/\/$/, '') + '/';
                    state.file.currentFiles = state.file.allFiles.filter(function (f) {
                        return f.path.indexOf(prefix) === 0 &&
                            f.path.substring(prefix.length).indexOf('/') === -1;
                    });
                }

                renderFilesView();
                updateProjectTree();
                updateBreadcrumb();
            } else {
                state.file.error = handleApiError(resp, 'Could not load files');
                content.innerHTML = '<div class="v4-workspace-empty"><h3>Could not load files</h3><p>' + esc(state.file.error) + '</p></div>';
            }
        }).catch(function () {
            state.file.loading = false;
            state.file.error = 'Network error';
            content.innerHTML = '<div class="v4-workspace-empty"><h3>Connection error</h3><p>Could not reach the server.</p></div>';
        });
    }

    function renderFilesView() {
        var content = $('workspaceContent');
        if (!content) return;
        var files = state.file.currentFiles || [];

        // Filter by search query
        var filtered = files;
        if (state.file.searchQuery) {
            var q = state.file.searchQuery.toLowerCase();
            filtered = files.filter(function (f) {
                return f.name.toLowerCase().indexOf(q) !== -1;
            });
        }

        // Sort
        filtered = filtered.slice().sort(function (a, b) {
            // Folders first
            if (a.type === 'folder' && b.type !== 'folder') return -1;
            if (a.type !== 'folder' && b.type === 'folder') return 1;
            var nameA = (a.name || '').toLowerCase();
            var nameB = (b.name || '').toLowerCase();
            if (state.file.sortOrder === 'size') return (a.size || 0) - (b.size || 0);
            if (state.file.sortOrder === 'date') return (b.updated_at || 0) - (a.updated_at || 0);
            return nameA.localeCompare(nameB);
        });

        if (filtered.length === 0) {
            content.innerHTML = '' +
                '<div class="v4-workspace-empty">' +
                '<h3>' + (state.file.searchQuery ? 'No matching files' : 'This folder is empty') + '</h3>' +
                '<p>' + (state.file.searchQuery ? 'Try a different search.' : 'Upload files or create a folder to get started.') + '</p>' +
                '</div>';
            return;
        }

        var viewMode = state.file.viewMode;
        if (viewMode === 'grid') {
            var grid = document.createElement('div');
            grid.className = 'v4-file-grid';
            filtered.forEach(function (f) {
                var card = document.createElement('div');
                card.className = 'v4-file-card' + (isSelected(f.id) ? ' selected' : '');
                card.dataset.fileId = f.id;
                card.innerHTML = '' +
                    '<div class="v4-file-card-icon">' + getIcon(f.name, f.type) + '</div>' +
                    '<div class="v4-file-card-name">' + esc(f.name) + '</div>' +
                    '<div class="v4-file-card-size">' + (f.type === 'folder' ? 'Folder' : fmtSize(f.size)) + '</div>';
                card.addEventListener('click', function (e) { handleFileClick(e, f); });
                card.addEventListener('dblclick', function () { openFile(f); });
                card.addEventListener('contextmenu', function (e) { e.preventDefault(); showContextMenu(e, f); });
                card.addEventListener('dragstart', function (e) { handleDragStart(e, f); });
                card.addEventListener('dragover', function (e) { handleDragOver(e, f); });
                card.addEventListener('drop', function (e) { handleDrop(e, f); });
                card.draggable = true;
                grid.appendChild(card);
            });
            content.innerHTML = '';
            content.appendChild(grid);
        } else {
            var list = document.createElement('div');
            list.className = 'v4-file-list';
            filtered.forEach(function (f) {
                var item = document.createElement('div');
                item.className = 'v4-file-list-item' + (isSelected(f.id) ? ' selected' : '');
                item.dataset.fileId = f.id;
                item.innerHTML = '' +
                    '<div class="v4-file-list-icon">' + getIcon(f.name, f.type) + '</div>' +
                    '<div class="v4-file-list-name">' + esc(f.name) + '</div>' +
                    '<div class="v4-file-list-size">' + (f.type === 'folder' ? 'Folder' : fmtSize(f.size)) + '</div>' +
                    '<div class="v4-file-list-date">' + fmtTime(f.updated_at) + '</div>';
                item.addEventListener('click', function (e) { handleFileClick(e, f); });
                item.addEventListener('dblclick', function () { openFile(f); });
                item.addEventListener('contextmenu', function (e) { e.preventDefault(); showContextMenu(e, f); });
                item.addEventListener('dragstart', function (e) { handleDragStart(e, f); });
                item.addEventListener('dragover', function (e) { handleDragOver(e, f); });
                item.addEventListener('drop', function (e) { handleDrop(e, f); });
                item.draggable = true;
                list.appendChild(item);
            });
            content.innerHTML = '';
            content.appendChild(list);
        }
    }

    // === File Selection ===
    function isSelected(fileId) {
        return state.file.selected.indexOf(fileId) !== -1;
    }

    function handleFileClick(e, file) {
        if (e.ctrlKey || e.metaKey) {
            // Toggle selection
            var idx = state.file.selected.indexOf(file.id);
            if (idx === -1) {
                state.file.selected.push(file.id);
            } else {
                state.file.selected.splice(idx, 1);
            }
        } else if (e.shiftKey && state.file.selected.length > 0) {
            // Shift selection: select range
            var files = state.file.currentFiles;
            var lastId = state.file.selected[state.file.selected.length - 1];
            var lastIdx = files.findIndex(function (f) { return f.id === lastId; });
            var thisIdx = files.findIndex(function (f) { return f.id === file.id; });
            if (lastIdx !== -1 && thisIdx !== -1) {
                var start = Math.min(lastIdx, thisIdx);
                var end = Math.max(lastIdx, thisIdx);
                state.file.selected = [];
                for (var i = start; i <= end; i++) {
                    state.file.selected.push(files[i].id);
                }
            }
        } else {
            // Single selection
            state.file.selected = [file.id];
        }
        renderFilesView();
    }

    // === Open File ===
    function openFile(file) {
        if (file.type === 'folder') {
            // Navigate into folder
            state.file.currentDir = file.path;
            state.file.selected = [];
            updateBreadcrumb();
            loadFiles(state.drop.current.id, file.path);
            return;
        }
        // Preview file
        previewFile(file);
    }

    function updateBreadcrumb() {
        var bc = $('breadcrumb');
        if (!bc || !state.drop.current) return;

        var parts = state.file.currentDir.split('/').filter(Boolean);
        var html = '<a href="/" class="v4-breadcrumb-item" id="bcHome">My Drops</a>' +
            '<span class="v4-breadcrumb-sep">/</span>' +
            '<a href="#" class="v4-breadcrumb-item" id="bcDrop">' + esc(state.drop.current.name) + '</a>';

        var currentPath = '';
        parts.forEach(function (part, i) {
            currentPath += '/' + part;
            html += '<span class="v4-breadcrumb-sep">/</span>';
            if (i === parts.length - 1) {
                html += '<span class="v4-breadcrumb-item active">' + esc(part) + '</span>';
            } else {
                html += '<a href="#" class="v4-breadcrumb-item" data-path="' + esc(currentPath) + '">' + esc(part) + '</a>';
            }
        });

        bc.innerHTML = html;

        var homeLink = $('bcHome');
        if (homeLink) homeLink.addEventListener('click', function (e) { e.preventDefault(); switchTab('home'); });

        var dropLink = $('bcDrop');
        if (dropLink) dropLink.addEventListener('click', function (e) {
            e.preventDefault();
            state.file.currentDir = '/';
            loadFiles(state.drop.current.id, '/');
        });

        qsa('[data-path]', bc).forEach(function (el) {
            el.addEventListener('click', function (e) {
                e.preventDefault();
                var p = el.getAttribute('data-path');
                state.file.currentDir = p;
                loadFiles(state.drop.current.id, p);
            });
        });
    }

    function previewFile(file) {
        if (!state.drop.current) return;
        showToast('Opening ' + file.name + '...', 'info');
        api.getFileContent(state.drop.current.id, file.id).then(function (data) {
            var text = typeof data === 'string' ? data : JSON.stringify(data, null, 2);
            if (text && text.length > 500) text = text.substring(0, 500) + '\n\n... (file truncated)';
            var content = $('workspaceContent');
            if (!content) return;
            content.innerHTML = '' +
                '<div class="v4-file-preview">' +
                '<div class="v4-file-preview-header">' +
                '<span class="v4-file-preview-title">' + esc(file.name) + '</span>' +
                '<div style="display:flex;gap:8px;">' +
                '<a href="' + CONFIG.apiBase + '/drop/' + state.drop.current.id + '/files/' + file.id + '/download" class="button-link small" target="_blank">Download</a>' +
                '<button type="button" class="button-link small" id="closePreview">Back to Files</button>' +
                '</div>' +
                '</div>' +
                '<pre class="v4-file-preview-content">' + esc(text || 'No content') + '</pre>' +
                '</div>';
            var closeBtn = $('closePreview');
            if (closeBtn) closeBtn.addEventListener('click', function () { renderFilesView(); });
        }).catch(function () {
            showToast('Could not read file content', 'error');
        });
    }

    // === Context Menu ===
    function showContextMenu(e, file) {
        // Close existing menu
        closeContextMenu();

        var canEdit = state.drop.permissions === 'editor' || state.drop.permissions === 'owner';
        var isFolder = file.type === 'folder';

        var menu = document.createElement('div');
        menu.className = 'v4-context-menu';
        menu.style.left = e.clientX + 'px';
        menu.style.top = e.clientY + 'px';

        var items = [];

        if (isFolder) {
            items.push({ label: 'Open', action: function () { openFile(file); } });
        } else {
            items.push({ label: 'Open', action: function () { openFile(file); } });
            items.push({ label: 'Download', action: function () {
                window.open(CONFIG.apiBase + '/drop/' + state.drop.current.id + '/files/' + file.id + '/download', '_blank');
            }});
        }

        if (canEdit) {
            items.push({ divider: true });
            items.push({ label: 'Rename', action: function () { showRenameDialog(file); } });
            items.push({ label: 'Move to...', action: function () { showMoveDialog(file); } });
            items.push({ divider: true });
            items.push({ label: 'Move to Trash', action: function () { trashItem(file); }, danger: true });
        }

        items.forEach(function (item) {
            if (item.divider) {
                var div = document.createElement('div');
                div.className = 'v4-context-menu-divider';
                menu.appendChild(div);
                return;
            }
            var btn = document.createElement('button');
            btn.type = 'button';
            btn.className = 'v4-context-menu-item' + (item.danger ? ' danger' : '');
            btn.textContent = item.label;
            btn.addEventListener('click', function () {
                closeContextMenu();
                item.action();
            });
            menu.appendChild(btn);
        });

        document.body.appendChild(menu);
        state.ui.contextMenu = menu;

        // Close on outside click
        setTimeout(function () {
            document.addEventListener('click', closeContextMenu, { once: true });
        }, 0);
    }

    function closeContextMenu() {
        if (state.ui.contextMenu) {
            state.ui.contextMenu.remove();
            state.ui.contextMenu = null;
        }
    }

    // === Drag and Drop ===
    function handleDragStart(e, file) {
        state.ui.dragSource = file;
        e.dataTransfer.effectAllowed = 'move';
        e.dataTransfer.setData('text/plain', file.id);
    }

    function handleDragOver(e, file) {
        e.preventDefault();
        e.dataTransfer.dropEffect = 'move';
        if (file.type === 'folder') {
            e.currentTarget.classList.add('drag-over');
        }
    }

    function handleDrop(e, targetFile) {
        e.preventDefault();
        e.currentTarget.classList.remove('drag-over');

        var source = state.ui.dragSource;
        if (!source || !state.drop.current) return;

        // Can only drop onto folders
        if (targetFile.type !== 'folder') return;

        // Can't drop into itself
        if (source.id === targetFile.id) return;

        // Can't drop a folder into its own subtree
        if (source.type === 'folder' && targetFile.path.indexOf(source.path + '/') === 0) {
            showToast('Cannot move a folder into itself', 'error');
            return;
        }

        var canEdit = state.drop.permissions === 'editor' || state.drop.permissions === 'owner';
        if (!canEdit) {
            showToast('You do not have permission to move files', 'error');
            return;
        }

        api.moveFile(state.drop.current.id, source.id, targetFile.path).then(function (resp) {
            if (resp.success) {
                showToast('Moved ' + source.name + ' to ' + targetFile.name, 'info');
                state.ui.dragSource = null;
                loadFiles(state.drop.current.id, state.file.currentDir);
            } else {
                showToast(handleApiError(resp, 'Failed to move item'), 'error');
            }
        });
    }

    // === Actions ===
    function trashItem(file) {
        if (!state.drop.current) return;
        if (confirm('Move "' + file.name + '" to Trash?')) {
            api.trashFile(state.drop.current.id, file.id).then(function (resp) {
                if (resp.success) {
                    showToast('Moved to Trash', 'info');
                    loadFiles(state.drop.current.id, state.file.currentDir);
                } else {
                    showToast(handleApiError(resp, 'Failed to move to trash'), 'error');
                }
            });
        }
    }

    function deleteItem(file) {
        if (!state.drop.current) return;
        if (confirm('Permanently delete "' + file.name + '"? This cannot be undone.')) {
            api.deleteFile(state.drop.current.id, file.id).then(function (resp) {
                if (resp.success) {
                    showToast('Deleted', 'info');
                    loadFiles(state.drop.current.id, state.file.currentDir);
                } else {
                    showToast(handleApiError(resp, 'Failed to delete'), 'error');
                }
            });
        }
    }

    // === Dialogs ===
    function showRenameDialog(file) {
        var dialog = $('renameDialog');
        var input = $('renameInput');
        var label = $('renameItemLabel');
        var error = $('renameError');

        if (!dialog || !input) return;
        input.value = file.name;
        if (label) label.textContent = 'Rename "' + file.name + '"';
        if (error) error.style.display = 'none';
        dialog._file = file;
        dialog.showModal();
        input.focus();
        input.select();
    }

    function showMoveDialog(file) {
        var dialog = $('moveDialog');
        var select = $('moveDestination');
        var label = $('moveItemLabel');
        var error = $('moveError');

        if (!dialog || !select) return;
        if (label) label.textContent = 'Move "' + file.name + '" to:';

        // Populate with all folders in the drop
        select.innerHTML = '<option value="/">/ (Root)</option>';
        (state.file.allFiles || []).forEach(function (f) {
            if (f.type === 'folder' && f.id !== file.id) {
                // Don't allow moving into own subtree
                if (file.type === 'folder' && f.path.indexOf(file.path + '/') === 0) return;
                var opt = document.createElement('option');
                opt.value = f.path;
                opt.textContent = '/' + f.path;
                select.appendChild(opt);
            }
        });

        if (error) error.style.display = 'none';
        dialog._file = file;
        dialog.showModal();
    }

    // === Drop Information Panel ===
    function updateDropPanel(drop) {
        var panel = $('dropPanelContent');
        if (!panel || !drop) return;

        panel.innerHTML = '' +
            '<div class="v4-drop-info">' +
            '<h3 class="v4-drop-name">' + esc(drop.name) + '</h3>' +
            '<dl class="v4-drop-meta">' +
            '<dt>Visibility</dt><dd><span class="v4-badge v4-badge-' + (drop.visibility || 'private') + '">' + (drop.visibility || 'private') + '</span></dd>' +
            '<dt>Created</dt><dd>' + fmtTime(drop.created_at) + '</dd>' +
            '<dt>Updated</dt><dd>' + fmtTime(drop.updated_at) + '</dd>' +
            (drop.description ? '<dt>Description</dt><dd>' + esc(drop.description) + '</dd>' : '') +
            '</dl>' +
            '<div class="v4-drop-actions">' +
            '<button type="button" class="v4-drop-action-btn" id="shareDropBtn"><svg viewBox="0 0 24 24" width="16" height="16"><path d="M4 12v8a2 2 0 002 2h12a2 2 0 002-2v-8" stroke="currentColor" stroke-width="2" fill="none"/><path d="M16 6l-4-4-4 4M12 2v13" stroke="currentColor" stroke-width="2" fill="none"/></svg> Share</button>' +
            '<button type="button" class="v4-drop-action-btn" id="copyLinkBtn"><svg viewBox="0 0 24 24" width="16" height="16"><path d="M10 13a5 5 0 007.54.54l3-3a5 5 0 00-7.07-7.07l-1.72 1.71" stroke="currentColor" stroke-width="2" fill="none"/><path d="M14 11a5 5 0 00-7.54-.54l-3 3a5 5 0 007.07 7.07l1.71-1.71" stroke="currentColor" stroke-width="2" fill="none"/></svg> Copy Link</button>' +
            '<button type="button" class="v4-drop-action-btn" id="manageMembersBtn"><svg viewBox="0 0 24 24" width="16" height="16"><path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2" stroke="currentColor" stroke-width="2" fill="none"/><circle cx="9" cy="7" r="4" stroke="currentColor" stroke-width="2" fill="none"/><path d="M23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75" stroke="currentColor" stroke-width="2" fill="none"/></svg> Members</button>' +
            '<button type="button" class="v4-drop-action-btn" id="settingsBtn"><svg viewBox="0 0 24 24" width="16" height="16"><circle cx="12" cy="12" r="3" stroke="currentColor" stroke-width="2" fill="none"/><path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 01-2.83 2.83l-.06-.06a1.65 1.65 0 00-1.82.33 1.65 1.65 0 00-.58 1.82 1.65 1.65 0 00.33 1.82l.06.06a2 2 0 01-2.83-2.83l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1.82.58 1.65 1.65 0 00-.33 1.82l-.06.06a2 2 0 01-2.83-2.83l.06-.06a1.65 1.65 0 00.33-1.82 1.65 1.65 0 00-1.82-.58 1.65 1.65 0 00-1.82.33zM12 8.5a3.5 3.5 0 100 7 3.5 3.5 0 000-7z" stroke="currentColor" stroke-width="2" fill="none"/></svg> Settings</button>' +
            '<button type="button" class="v4-drop-action-btn" id="activityBtn"><svg viewBox="0 0 24 24" width="16" height="16"><circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" fill="none"/><path d="M12 6v6l4 2" stroke="currentColor" stroke-width="2" fill="none"/></svg> Activity</button>' +
            '<button type="button" class="v4-drop-action-btn v4-drop-action-danger" id="deleteBtn" data-action="delete-drop-panel"><svg viewBox="0 0 24 24" width="16" height="16"><polyline points="3 6 5 6 21 6" stroke="currentColor" stroke-width="2" fill="none"/><path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2" stroke="currentColor" stroke-width="2" fill="none"/></svg> Delete</button>' +
            '</div></div>';

        var shareBtn = $('shareDropBtn');
        if (shareBtn) shareBtn.addEventListener('click', function () { showShareDialog(drop); });
        var copyBtn = $('copyLinkBtn');
        if (copyBtn) copyBtn.addEventListener('click', function () { copyDropLink(drop); });
        var membersBtn = $('manageMembersBtn');
        if (membersBtn) membersBtn.addEventListener('click', function () { showMemberManageDialog(drop); });
        var settingsBtn = $('settingsBtn');
        if (settingsBtn) settingsBtn.addEventListener('click', function () { showDropSettings(drop); });
        var actBtn = $('activityBtn');
        if (actBtn) actBtn.addEventListener('click', function () { showActivity(drop.id); });

        updatePermissionUI();
    }

    // === Share Dialog ===
    function showShareDialog(drop) {
        var dialog = $('shareDropDialog');
        var nameEl = $('shareDropName');
        var contactInput = $('shareContactName');
        var suggestions = $('shareContactSuggestions');

        if (!dialog) return;
        if (nameEl) nameEl.textContent = 'Share "' + drop.name + '" with a contact';

        // Load contacts for suggestions
        api.getContacts().then(function (resp) {
            if (resp.success && resp.data && suggestions) {
                suggestions.innerHTML = '';
                resp.data.forEach(function (c) {
                    var opt = document.createElement('option');
                    opt.value = c.name;
                    suggestions.appendChild(opt);
                });
            }
        });

        if (contactInput) contactInput.value = '';
        var errorEl = $('shareDropError');
        if (errorEl) errorEl.style.display = 'none';
        dialog.showModal();
    }

    function showDropSettings(drop) {
        var dialog = $('dropSettingsDialog');
        if (!dialog || !drop) return;
        
        var nameEl = $('settingDropName');
        var descEl = $('settingDropDescription');
        var visEl = $('settingDropVisibility');
        var errorEl = $('dropSettingsError');
        
        if (nameEl) nameEl.value = drop.name || '';
        if (descEl) descEl.value = drop.description || '';
        if (visEl) visEl.value = drop.visibility || 'private';
        if (errorEl) errorEl.style.display = 'none';
        
        dialog.dataset.dropId = drop.id;
        dialog.showModal();
    }

    function saveDropSettings(e) {
        e.preventDefault();
        var dialog = $('dropSettingsDialog');
        var dropID = dialog.dataset.dropId;
        var nameEl = $('settingDropName');
        var descEl = $('settingDropDescription');
        var visEl = $('settingDropVisibility');
        var errorEl = $('dropSettingsError');
        
        var data = {
            name: nameEl ? nameEl.value.trim() : '',
            description: descEl ? descEl.value.trim() : '',
            visibility: visEl ? visEl.value : 'private'
        };
        
        if (!data.name) {
            if (errorEl) {
                errorEl.textContent = 'Drop name is required';
                errorEl.style.display = 'block';
            }
            return;
        }
        
        api.updateDrop(dropID, data).then(function (resp) {
            if (resp.success) {
                showToast('Drop settings saved', 'success');
                dialog.close();
                if (state.drop.current && state.drop.current.id === dropID) {
                    state.drop.current.name = data.name;
                    state.drop.current.description = data.description;
                    state.drop.current.visibility = data.visibility;
                    updateDropPanel(state.drop.current);
                }
            } else {
                if (errorEl) {
                    errorEl.textContent = handleApiError(resp, 'Failed to save settings');
                    errorEl.style.display = 'block';
                }
            }
        }).catch(function () {
            if (errorEl) {
                errorEl.textContent = 'Network error. Please try again.';
                errorEl.style.display = 'block';
            }
        });
    }

    function showDeleteDropDialog(drop) {
        var dialog = $('deleteDropDialog');
        if (!dialog || !drop) return;
        
        var nameEl = $('deleteDropName');
        if (nameEl) nameEl.textContent = drop.name;
        
        dialog.dataset.dropId = drop.id;
        dialog.showModal();
    }

    function deleteDropConfirmed(e) {
        e.preventDefault();
        var dialog = $('deleteDropDialog');
        var dropID = dialog.dataset.dropId;
        
        api.deleteDrop(dropID).then(function (resp) {
            if (resp.success) {
                showToast('Drop deleted', 'success');
                dialog.close();
                state.drop.current = null;
                switchTab('home');
            } else {
                showToast(handleApiError(resp, 'Failed to delete drop'), 'error');
            }
        }).catch(function () {
            showToast('Network error. Please try again.', 'error');
        });
    }
    // === Member Management ===
    function showMemberManageDialog(drop) {
        var dialog = $('memberManageDialog');
        if (!dialog || !drop) return;
        
        dialog.dataset.dropId = drop.id;
        renderMembersList(drop.id);
        dialog.showModal();
    }

    function renderMembersList(dropID) {
        var container = $('memberList');
        if (!container) return;
        
        var members = state.drop.members || [];
        var currentUserID = getCurrentUserId();
        var currentUserRole = state.drop.permissions;
        
        if (members.length === 0) {
            container.innerHTML = '<p class="subtle">No members yet.</p>';
            return;
        }
        
        container.innerHTML = '';
        members.forEach(function (member) {
            var isOwner = member.role === 'owner';
            var isCurrentUser = member.user_id === currentUserID;
            var canManage = currentUserRole === 'owner' && !isOwner;
            
            var item = document.createElement('div');
            item.className = 'v4-member-item';
            
            var avatarLetter = (member.user_name || 'U').charAt(0).toUpperCase();
            
            var actionsHTML = '';
            if (canManage) {
                actionsHTML = '<div class="v4-member-actions">' +
                    '<select data-user-id="' + member.user_id + '">' +
                    '<option value="viewer"' + (member.role === 'viewer' ? ' selected' : '') + '>Viewer</option>' +
                    '<option value="editor"' + (member.role === 'editor' ? ' selected' : '') + '>Editor</option>' +
                    '<option value="owner"' + (member.role === 'owner' ? ' selected' : '') + '>Owner</option>' +
                    '</select>' +
                    '<button data-user-id="' + member.user_id + '" data-action="remove-member" class="danger">Remove</button>' +
                    '</div>';
            } else if (isCurrentUser && !isOwner) {
                actionsHTML = '<div class="v4-member-actions">' +
                    '<button data-user-id="' + member.user_id + '" data-action="leave-drop" class="danger">Leave Drop</button>' +
                    '</div>';
            }
            
            item.innerHTML = '' +
                '<div class="v4-member-info">' +
                '<div class="v4-member-avatar">' + avatarLetter + '</div>' +
                '<div class="v4-member-details">' +
                '<div class="v4-member-name">' + esc(member.user_name || ('User ' + member.user_id)) + (isCurrentUser ? ' (you)' : '') + '</div>' +
                '<div class="v4-member-role">' + member.role.charAt(0).toUpperCase() + member.role.slice(1) + (isOwner ? ' · Owner' : '') + '</div>' +
                '</div>' +
                '</div>' +
                actionsHTML;
            
            container.appendChild(item);
        });
        
        container.querySelectorAll('select').forEach(function (select) {
            select.addEventListener('change', function () {
                var userId = parseInt(select.dataset.userId);
                var newRole = select.value;
                changeMemberRole(dropID, userId, newRole);
            });
        });
        
        container.querySelectorAll('button[data-action="remove-member"]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                var userId = parseInt(btn.dataset.userId);
                if (confirm('Remove this member from the drop?')) {
                    removeMember(dropID, userId);
                }
            });
        });
        
        container.querySelectorAll('button[data-action="leave-drop"]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                var userId = parseInt(btn.dataset.userId);
                if (confirm('Are you sure you want to leave this drop? You will lose access.')) {
                    leaveDrop(dropID, userId);
                }
            });
        });
    }

    function changeMemberRole(dropID, userId, newRole) {
        api.updateMemberRole(dropID, userId, newRole).then(function (resp) {
            if (resp.success) {
                showToast('Member role updated', 'success');
                api.getMembers(dropID).then(function (resp) {
                    if (resp.success) {
                        state.drop.members = resp.data || [];
                        renderMembersList(dropID);
                    }
                });
            } else {
                showToast(handleApiError(resp, 'Failed to update role'), 'error');
            }
        }).catch(function () {
            showToast('Network error. Please try again.', 'error');
        });
    }
    function removeMember(dropID, userId) {
        api.removeMember(dropID, userId).then(function (resp) {
            if (resp.success) {
                showToast('Member removed', 'success');
                api.getMembers(dropID).then(function (resp) {
                    if (resp.success) {
                        state.drop.members = resp.data || [];
                        renderMembersList(dropID);
                    }
                });
            } else {
                showToast(handleApiError(resp, 'Failed to remove member'), 'error');
            }
        }).catch(function () {
            showToast('Network error. Please try again.', 'error');
        });
    }

    function leaveDrop(dropID, userId) {
        api.removeMember(dropID, userId).then(function (resp) {
            if (resp.success) {
                showToast('You left the drop', 'success');
                $('memberManageDialog').close();
                state.drop.current = null;
                switchTab('home');
            } else {
                showToast(handleApiError(resp, 'Failed to leave drop'), 'error');
            }
        }).catch(function () {
            showToast('Network error. Please try again.', 'error');
        });
    }

    function showAddMemberDialog(drop) {
        var dialog = $('addMemberDialog');
        if (!dialog || !drop) return;
        
        var nameEl = $('addMemberName');
        var roleEl = $('addMemberRole');
        var errorEl = $('addMemberError');
        var suggestions = $('userSuggestions');
        
        if (nameEl) nameEl.value = '';
        if (roleEl) roleEl.value = 'viewer';
        if (errorEl) errorEl.style.display = 'none';
        
        api.getContacts().then(function (resp) {
            if (resp.success && resp.data && suggestions) {
                suggestions.innerHTML = '';
                resp.data.forEach(function (c) {
                    var opt = document.createElement('option');
                    opt.value = c.name;
                    suggestions.appendChild(opt);
                });
            }
        });
        
        dialog.dataset.dropId = drop.id;
        dialog.showModal();
    }

    function addMemberConfirmed(e) {
        e.preventDefault();
        var dialog = $('addMemberDialog');
        var dropID = dialog.dataset.dropId;
        var nameEl = $('addMemberName');
        var roleEl = $('addMemberRole');
        var errorEl = $('addMemberError');
        
        var userName = nameEl ? nameEl.value.trim() : '';
        var role = roleEl ? roleEl.value : 'viewer';
        
        if (!userName) {
            if (errorEl) {
                errorEl.textContent = 'Username is required';
                errorEl.style.display = 'block';
            }
            return;
        }
        
        api.shareDrop(dropID, userName, role).then(function (resp) {
            if (resp.success) {
                showToast('Member added', 'success');
                dialog.close();
                api.getMembers(dropID).then(function (resp) {
                    if (resp.success) {
                        state.drop.members = resp.data || [];
                    }
                });
            } else {
                if (errorEl) {
                    errorEl.textContent = handleApiError(resp, 'Failed to add member');
                    errorEl.style.display = 'block';
                }
            }
        }).catch(function () {
            if (errorEl) {
                errorEl.textContent = 'Network error. Please try again.';
                errorEl.style.display = 'block';
            }
        });
    }

    function copyDropLink(drop) {
        if (!drop) return;
        var link = window.location.origin + '/browse/' + (drop.owner_name || '') + '/' + drop.name;
        navigator.clipboard.writeText(link).then(function () {
            showToast('Link copied to clipboard', 'success');
        }).catch(function () {
            showToast('Failed to copy link', 'error');
        });
    }




    // === Shared/Public Views ===
    function renderSharedView() {
        var content = $('workspaceContent');
        if (!content) return;
        content.innerHTML = '<div class="v4-workspace-empty"><p>Loading shared drops...</p></div>';

        api.listDrops().then(function (resp) {
            if (!resp.success) {
                content.innerHTML = '<div class="v4-workspace-empty"><h3>Could not load</h3></div>';
                return;
            }
            var drops = resp.data || [];
            // Filter to drops where user is not the owner
            var shared = drops.filter(function (d) {
                return d.owner_id !== getCurrentUserId();
            });
            if (shared.length === 0) {
                content.innerHTML = '' +
                    '<div class="v4-workspace-empty">' +
                    '<h3>Shared With Me</h3>' +
                    '<p>Drops that others have shared with you will appear here.</p>' +
                    '<p class="subtle">Use the Share button on any Drop to invite collaborators.</p>' +
                    '</div>';
                return;
            }
            state.sharedDrops = shared;
            var grid = document.createElement('div');
            grid.className = 'v4-drop-grid';
            shared.forEach(function (drop) {
                var card = document.createElement('div');
                card.className = 'v4-drop-card';
                card.innerHTML = '' +
                    '<h4 class="v4-drop-card-name">' + esc(drop.name) + '</h4>' +
                    '<p class="v4-drop-card-desc">' + (drop.description ? esc(drop.description) : 'No description') + '</p>' +
                    '<div class="v4-drop-card-meta">' +
                    '<span class="v4-badge v4-badge-' + (drop.visibility || 'private') + '">' + (drop.visibility || 'private') + '</span>' +
                    '<span>' + fmtTime(drop.updated_at) + '</span>' +
                    '</div>';
                card.addEventListener('click', function () { openDrop(drop); });
                grid.appendChild(card);
            });
            content.innerHTML = '';
            content.appendChild(grid);
        });
    }

    function renderPublicView() {
        var content = $('workspaceContent');
        if (!content) return;
        content.innerHTML = '<div class="v4-workspace-empty"><p>Loading public drops...</p></div>';

        api.listDrops().then(function (resp) {
            if (!resp.success) {
                content.innerHTML = '<div class="v4-workspace-empty"><h3>Could not load</h3></div>';
                return;
            }
            var drops = resp.data || [];
            var publicDrops = drops.filter(function (d) {
                return d.visibility === 'public';
            });
            if (publicDrops.length === 0) {
                content.innerHTML = '' +
                    '<div class="v4-workspace-empty">' +
                    '<h3>Public Drops</h3>' +
                    '<p>Community drops will be shown here.</p>' +
                    '</div>';
                return;
            }
            state.publicDrops = publicDrops;
            var grid = document.createElement('div');
            grid.className = 'v4-drop-grid';
            publicDrops.forEach(function (drop) {
                var card = document.createElement('div');
                card.className = 'v4-drop-card';
                card.innerHTML = '' +
                    '<h4 class="v4-drop-card-name">' + esc(drop.name) + '</h4>' +
                    '<p class="v4-drop-card-desc">' + (drop.description ? esc(drop.description) : 'No description') + '</p>' +
                    '<div class="v4-drop-card-meta">' +
                    '<span class="v4-badge v4-badge-public">public</span>' +
                    '<span>' + fmtTime(drop.updated_at) + '</span>' +
                    '</div>';
                card.addEventListener('click', function () { openDrop(drop); });
                grid.appendChild(card);
            });
            content.innerHTML = '';
            content.appendChild(grid);
        });
    }

    // === Contacts View ===
    function renderContactsView() {
        var content = $('workspaceContent');
        if (!content) return;
        content.innerHTML = '<div class="v4-workspace-empty"><p>Loading contacts...</p></div>';

        api.getContacts().then(function (resp) {
            var contacts = (resp.success && resp.data) ? resp.data : [];
            renderContactsPage(content, contacts);
        }).catch(function () {
            content.innerHTML = '<div class="v4-workspace-empty"><h3>Could not load contacts</h3></div>';
        });
    }

    function renderContactsPage(container, contacts) {
        container.innerHTML = '';

        // Find people section
        var findSection = document.createElement('div');
        findSection.className = 'v4-contacts-section';
        findSection.innerHTML = '' +
            '<h3>Find People</h3>' +
            '<div style="display:flex;gap:8px;">' +
            '<input type="search" id="findPeopleInput" class="details" placeholder="Search by username..." style="flex:1;" />' +
            '<button type="button" class="redirect" id="searchPeopleBtn">Search</button>' +
            '</div>' +
            '<div id="peopleResults" class="v4-contacts-list"></div>';
        container.appendChild(findSection);

        // My contacts section
        var contactsSection = document.createElement('div');
        contactsSection.className = 'v4-contacts-section';
        contactsSection.innerHTML = '<h3>My Contacts (' + contacts.length + ')</h3><div id="myContactsList" class="v4-contacts-list"></div>';
        container.appendChild(contactsSection);

        // Render contacts
        var listEl = $('myContactsList');
        if (listEl) {
            if (contacts.length === 0) {
                listEl.innerHTML = '<div class="subtle" style="padding:12px 0;">No contacts yet. Search for people to add.</div>';
            } else {
                contacts.forEach(function (contact) {
                    var card = document.createElement('div');
                    card.className = 'v4-contact-card';
                    card.innerHTML = '' +
                        '<div class="v4-contact-avatar">' +
                        (contact.pfp ? '<img src="' + esc(contact.pfp) + '" alt="" style="width:32px;height:32px;border-radius:999px;object-fit:cover;" />' :
                        '<svg viewBox="0 0 24 24" width="20" height="20" fill="var(--muted)"><circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0116 0"/></svg>') +
                        '</div>' +
                        '<div class="v4-contact-name">' + esc(contact.name) + '</div>' +
                        '<button type="button" class="button-link small danger" data-remove="' + esc(contact.name) + '">Remove</button>';
                    var removeBtn = card.querySelector('[data-remove]');
                    if (removeBtn) {
                        removeBtn.addEventListener('click', function () {
                            api.removeContact(contact.name).then(function (resp) {
                                if (resp.success) { showToast('Contact removed', 'info'); renderContactsView(); }
                                else { showToast(handleApiError(resp, 'Could not remove contact'), 'error'); }
                            });
                        });
                    }
                    listEl.appendChild(card);
                });
            }
        }

        // Search people
        var searchInput = $('findPeopleInput');
        var searchBtn = $('searchPeopleBtn');
        var resultsEl = $('peopleResults');

        function doSearch() {
            if (!resultsEl || !searchInput) return;
            var q = searchInput.value.trim();
            if (!q) { resultsEl.innerHTML = ''; return; }
            resultsEl.innerHTML = '<div class="subtle">Searching...</div>';
            api.searchUsers(q).then(function (resp) {
                if (!resp.success || !resp.data || resp.data.length === 0) {
                    resultsEl.innerHTML = '<div class="subtle">No users found.</div>';
                    return;
                }
                resultsEl.innerHTML = '';
                resp.data.forEach(function (user) {
                    var card = document.createElement('div');
                    card.className = 'v4-contact-card';
                    var isContact = contacts.some(function (c) { return c.name === user.name; });
                    var status = user.status || '';
                    card.innerHTML = '' +
                        '<div class="v4-contact-avatar"><img src="' + (user.pfp || '/pfp/default.png') + '" alt="" style="width:32px;height:32px;border-radius:999px;object-fit:cover;" /></div>' +
                        '<div class="v4-contact-name">' + esc(user.name) + '</div>';
                    if (status === 'pending') {
                        card.innerHTML += '<span class="status-pill">Pending</span>';
                    } else if (status === 'accepted' || isContact) {
                        card.innerHTML += '<span class="status-pill" style="background:rgba(22,163,74,0.15);color:var(--green);">Contact</span>';
                    } else {
                        var addBtn = document.createElement('button');
                        addBtn.type = 'button';
                        addBtn.className = 'button-link small';
                        addBtn.textContent = 'Add';
                        addBtn.addEventListener('click', function () {
                            addBtn.disabled = true;
                            addBtn.textContent = 'Sending...';
                            api.addContact(user.name).then(function (r) {
                                if (r.success) { showToast('Request sent to ' + user.name, 'info'); doSearch(); }
                                else { showToast(handleApiError(r, 'Could not send request'), 'error'); addBtn.disabled = false; addBtn.textContent = 'Add'; }
                            });
                        });
                        card.appendChild(addBtn);
                    }
                    resultsEl.appendChild(card);
                });
            });
        }

        if (searchBtn) searchBtn.addEventListener('click', doSearch);
        if (searchInput) searchInput.addEventListener('keydown', function (e) { if (e.key === 'Enter') doSearch(); });
    }

    // === Activity ===
    function showActivity(dropID) {
        api.getActivity(dropID).then(function (resp) {
            if (!resp.success || !resp.data || resp.data.length === 0) {
                showToast('No recent activity', 'info');
                return;
            }
            var text = 'Recent Activity:\n';
            resp.data.forEach(function (log) {
                text += '\n• ' + (log.event_type || 'event') + ' — ' + fmtTime(log.created_at);
            });
            showToast(text.substring(0, 200), 'info');
        });
    }

    // === Project Tree ===
    function updateProjectTree() {
        var files = state.file.allFiles || [];
        var fileTree = $('fileTree');
        var docTree = $('docTree');
        var trashTree = $('trashTree');

        function renderTree(container, items, icon) {
            if (!container) return;
            container.innerHTML = '';
            if (items.length === 0) {
                container.innerHTML = '<span class="v4-tree-item" style="color:var(--muted);font-size:12px;cursor:default;">Empty</span>';
                return;
            }
            items.forEach(function (f) {
                var item = document.createElement('a');
                item.className = 'v4-tree-item file';
                item.href = '#';
                item.innerHTML = '<span class="v4-tree-item-icon">' + icon + '</span>' + esc(f.name);
                item.addEventListener('click', function (e) { e.preventDefault(); openFile(f); });
                container.appendChild(item);
            });
        }

        // Only show top-level items in the tree
        var topLevel = files.filter(function (f) {
            return f.path.indexOf('/') === -1;
        });

        renderTree(fileTree, topLevel.filter(function (f) { return f.type === 'file' || !f.type; }), '<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8l-6-6z" fill="none" stroke="currentColor" stroke-width="2"/><path d="M14 2v6h6" fill="none" stroke="currentColor" stroke-width="2"/></svg>');
        renderTree(docTree, topLevel.filter(function (f) { return f.type === 'document'; }), '<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8l-6-6z" fill="none" stroke="currentColor" stroke-width="2"/><path d="M14 2v6h6M16 13H8M16 17H8M10 9H8" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>');

        // Trash tree
        if (trashTree) {
            trashTree.innerHTML = '';
            var trashBtn = document.createElement('a');
            trashBtn.className = 'v4-tree-item file';
            trashBtn.href = '#';
            trashBtn.innerHTML = '<span class="v4-tree-item-icon"><svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M3 6h18M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg></span>View Trash';
            trashBtn.addEventListener('click', function (e) {
                e.preventDefault();
                showTrashView();
            });
            trashTree.appendChild(trashBtn);
        }
    }

    function showTrashView() {
        if (!state.drop.current) return;
        var content = $('workspaceContent');
        if (!content) return;
        content.innerHTML = '<div class="v4-workspace-empty"><p>Loading trash...</p></div>';

        api.listTrash(state.drop.current.id).then(function (resp) {
            if (!resp.success) {
                content.innerHTML = '<div class="v4-workspace-empty"><h3>Could not load trash</h3></div>';
                return;
            }
            var trashed = resp.data || [];
            if (trashed.length === 0) {
                content.innerHTML = '' +
                    '<div class="v4-workspace-empty">' +
                    '<h3>Trash is empty</h3>' +
                    '<p>Deleted items will appear here.</p>' +
                    '</div>';
                return;
            }

            var list = document.createElement('div');
            list.className = 'v4-file-list';
            trashed.forEach(function (f) {
                var item = document.createElement('div');
                item.className = 'v4-file-list-item';
                item.innerHTML = '' +
                    '<div class="v4-file-list-icon">' + getIcon(f.name, f.type) + '</div>' +
                    '<div class="v4-file-list-name">' + esc(f.name) + '</div>' +
                    '<div class="v4-file-list-size">' + (f.type === 'folder' ? 'Folder' : fmtSize(f.size)) + '</div>' +
                    '<div class="v4-file-list-date">' + fmtTime(f.updated_at) + '</div>';
                var actions = document.createElement('div');
                actions.style.display = 'flex';
                actions.style.gap = '8px';

                var restoreBtn = document.createElement('button');
                restoreBtn.type = 'button';
                restoreBtn.className = 'button-link small';
                restoreBtn.textContent = 'Restore';
                restoreBtn.addEventListener('click', function () {
                    api.restoreFile(state.drop.current.id, f.id).then(function (resp) {
                        if (resp.success) {
                            showToast('Restored ' + f.name, 'info');
                            showTrashView();
                        } else {
                            showToast(handleApiError(resp, 'Failed to restore'), 'error');
                        }
                    });
                });
                actions.appendChild(restoreBtn);

                var deleteBtn = document.createElement('button');
                deleteBtn.type = 'button';
                deleteBtn.className = 'button-link small danger';
                deleteBtn.textContent = 'Delete';
                deleteBtn.addEventListener('click', function () {
                    if (confirm('Permanently delete "' + f.name + '"?')) {
                        api.deleteFile(state.drop.current.id, f.id).then(function (resp) {
                            if (resp.success) {
                                showToast('Deleted permanently', 'info');
                                showTrashView();
                            } else {
                                showToast(handleApiError(resp, 'Failed to delete'), 'error');
                            }
                        });
                    }
                });
                actions.appendChild(deleteBtn);

                item.appendChild(actions);
                list.appendChild(item);
            });

            content.innerHTML = '';
            content.appendChild(list);

            // Empty trash button
            var emptyBtn = document.createElement('button');
            emptyBtn.type = 'button';
            emptyBtn.className = 'button-link small danger';
            emptyBtn.textContent = 'Empty Trash';
            emptyBtn.style.marginTop = '12px';
            emptyBtn.addEventListener('click', function () {
                if (confirm('Empty the trash? This permanently deletes all items.')) {
                    api.emptyTrash(state.drop.current.id).then(function (resp) {
                        if (resp.success) {
                            showToast('Trash emptied', 'info');
                            showTrashView();
                        } else {
                            showToast(handleApiError(resp, 'Failed to empty trash'), 'error');
                        }
                    });
                }
            });
            content.appendChild(emptyBtn);
        });
    }

    // === Actions ===
    function initActions() {
        // New Drop button
        qsa('[data-action="new-drop"]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                var d = $('createDropDialog');
                if (d) d.showModal();
            });
        });

        // Upload
        qsa('[data-action="upload"]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                if (state.drop.current) {
                    var d = $('uploadDialog');
                    if (d) d.showModal();
                } else {
                    showToast('Select a Drop first', 'warning');
                }
            });
        });

        // New Folder
        qsa('[data-action="new-folder"]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                if (state.drop.current) {
                    var d = $('newFolderDialog');
                    if (d) d.showModal();
                } else {
                    showToast('Select a Drop first', 'warning');
                }
            });
        });

        // Delete Drop
        qsa('[data-action="delete-drop"]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                if (!state.drop.current) {
                    showToast('Select a Drop first', 'warning');
                    return;
                }
                if (confirm('Delete "' + state.drop.current.name + '"? This cannot be undone.')) {
                    api.deleteDrop(state.drop.current.id).then(function (resp) {
                        if (resp.success) {
                            showToast('Drop deleted', 'info');
                            state.drop.current = null;
                            state.file.allFiles = [];
                            state.file.currentFiles = [];
                            var panel = $('dropPanelContent');
                            if (panel) panel.innerHTML = '<div class="v4-drop-panel-placeholder"><svg viewBox="0 0 24 24" width="32" height="32" stroke="var(--muted)" stroke-width="1.5" fill="none"><path d="M6 3h9l4 4v14H6z"/><path d="M15 3v5h5"/></svg><p>Select a Drop<br/>to view details</p></div>';
                            switchTab('home');
                        } else {
                            showToast(handleApiError(resp, 'Could not delete drop'), 'error');
                        }
                    });
                }
            });
        // Drop Settings
        qsa('[data-action="drop-settings"]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                if (state.drop.current) {
                    showDropSettings(state.drop.current);
                }
            });
        });

        // Manage Members
        qsa('[data-action="manage-members"]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                if (state.drop.current) {
                    showMemberManageDialog(state.drop.current);
                }
            });
        });

        // Add Member
        qsa('[data-action="add-member"]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                if (state.drop.current) {
                    showAddMemberDialog(state.drop.current);
                }
            });
        });

        // Copy Drop Link
        qsa('[data-action="copy-link"]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                if (state.drop.current) {
                    copyDropLink(state.drop.current);
                }
            });
        });

        // Delete Drop (from panel button)
        qsa('[data-action="delete-drop-panel"]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                if (state.drop.current) {
                    showDeleteDropDialog(state.drop.current);
                }
            });
        });

        });
    }

    // === Dialog Initialization ===
    function initDialogs() {
        // Create Drop
        var createDialog = $('createDropDialog');
        var cancelCreate = $('cancelCreateDrop');
        var createForm = $('createDropForm');
        var createError = $('createDropError');

        if (cancelCreate && createDialog) {
            cancelCreate.addEventListener('click', function () { createDialog.close(); if (createError) createError.style.display = 'none'; });
        }
        if (createForm && createDialog) {
            createForm.addEventListener('submit', function (e) {
                e.preventDefault();
                var name = $('newDropName');
                var desc = $('newDropDesc');
                var submitBtn = $('submitCreateDrop');
                if (!name || !name.value.trim()) return;
                if (submitBtn) submitBtn.disabled = true;
                if (createError) createError.style.display = 'none';

                api.createDrop(name.value.trim(), desc ? desc.value.trim() : '').then(function (resp) {
                    if (submitBtn) submitBtn.disabled = false;
                    if (resp.success) {
                        createDialog.close();
                        if (name) name.value = '';
                        if (desc) desc.value = '';
                        showToast('Drop "' + resp.data.name + '" created!', 'info');
                        switchTab('home');
                    } else {
                        var msg = handleApiError(resp, 'Unknown error');
                        if (createError) {
                            createError.textContent = 'Error: ' + msg;
                            createError.style.display = 'block';
                        }
                        showToast('Failed: ' + msg, 'error');
                    }
                }).catch(function () {
                    if (submitBtn) submitBtn.disabled = false;
                    showToast('Network error', 'error');
                });
            });
        }

        // New Folder
        var folderDialog = $('newFolderDialog');
        var cancelFolder = $('cancelNewFolder');
        var folderForm = $('newFolderForm');
        var folderError = $('newFolderError');

        if (cancelFolder && folderDialog) {
            cancelFolder.addEventListener('click', function () { folderDialog.close(); if (folderError) folderError.style.display = 'none'; });
        }
        if (folderForm && folderDialog) {
            folderForm.addEventListener('submit', function (e) {
                e.preventDefault();
                var nameInput = $('newFolderName');
                var submitBtn = $('submitNewFolder');
                if (!nameInput || !nameInput.value.trim() || !state.drop.current) return;
                if (submitBtn) submitBtn.disabled = true;
                if (folderError) folderError.style.display = 'none';

                api.createFolder(state.drop.current.id, nameInput.value.trim(), state.file.currentDir).then(function (resp) {
                    if (submitBtn) submitBtn.disabled = false;
                    if (resp.success) {
                        folderDialog.close();
                        if (nameInput) nameInput.value = '';
                        showToast('Folder created', 'info');
                        loadFiles(state.drop.current.id, state.file.currentDir);
                    } else {
                        var msg = handleApiError(resp, 'Failed to create folder');
                        if (folderError) {
                            folderError.textContent = 'Error: ' + msg;
                            folderError.style.display = 'block';
                        }
                        showToast('Failed: ' + msg, 'error');
                    }
                });
            });
            });
        }

        // Drop Settings
        var settingsForm = $('dropSettingsForm');
        var cancelSettings = $('cancelDropSettings');
        var settingsDialog = $('dropSettingsDialog');
        if (cancelSettings && settingsDialog) {
            cancelSettings.addEventListener('click', function () { settingsDialog.close(); });
        }
        if (settingsForm) {
            settingsForm.addEventListener('submit', saveDropSettings);
        }

        // Delete Drop
        var deleteForm = $('deleteDropForm');
        var cancelDelete = $('cancelDeleteDrop');
        var deleteDialog = $('deleteDropDialog');
        if (cancelDelete && deleteDialog) {
            cancelDelete.addEventListener('click', function () { deleteDialog.close(); });
        }
        if (deleteForm) {
            deleteForm.addEventListener('submit', deleteDropConfirmed);
        }

        // Member Management
        var memberDialog = $('memberManageDialog');
        var closeMember = $('closeMemberManage');
        if (closeMember && memberDialog) {
            closeMember.addEventListener('click', function () { memberDialog.close(); });
        }

        // Add Member
        var addMemberForm = $('addMemberForm');
        var cancelAddMember = $('cancelAddMember');
        var addMemberDialog = $('addMemberDialog');
        if (cancelAddMember && addMemberDialog) {
            cancelAddMember.addEventListener('click', function () { addMemberDialog.close(); });
        }
        if (addMemberForm) {
            addMemberForm.addEventListener('submit', addMemberConfirmed);
        }

        }

        // Rename
        var renameDialog = $('renameDialog');
        var cancelRename = $('cancelRename');
        var renameForm = $('renameForm');
        var renameError = $('renameError');

        if (cancelRename && renameDialog) {
            cancelRename.addEventListener('click', function () { renameDialog.close(); if (renameError) renameError.style.display = 'none'; });
        }
        if (renameForm && renameDialog) {
            renameForm.addEventListener('submit', function (e) {
                e.preventDefault();
                var input = $('renameInput');
                var submitBtn = $('submitRename');
                var file = renameDialog._file;
                if (!input || !input.value.trim() || !file || !state.drop.current) return;
                if (submitBtn) submitBtn.disabled = true;
                if (renameError) renameError.style.display = 'none';

                api.renameFile(state.drop.current.id, file.id, input.value.trim()).then(function (resp) {
                    if (submitBtn) submitBtn.disabled = false;
                    if (resp.success) {
                        renameDialog.close();
                        showToast('Renamed to ' + resp.data.name, 'info');
                        loadFiles(state.drop.current.id, state.file.currentDir);
                    } else {
                        var msg = handleApiError(resp, 'Failed to rename');
                        if (renameError) {
                            renameError.textContent = 'Error: ' + msg;
                            renameError.style.display = 'block';
                        }
                        showToast('Failed: ' + msg, 'error');
                    }
                });
            });
        }

        // Move
        var moveDialog = $('moveDialog');
        var cancelMove = $('cancelMove');
        var moveForm = $('moveForm');
        var moveError = $('moveError');

        if (cancelMove && moveDialog) {
            cancelMove.addEventListener('click', function () { moveDialog.close(); if (moveError) moveError.style.display = 'none'; });
        }
        if (moveForm && moveDialog) {
            moveForm.addEventListener('submit', function (e) {
                e.preventDefault();
                var dest = $('moveDestination');
                var submitBtn = $('submitMove');
                var file = moveDialog._file;
                if (!dest || !file || !state.drop.current) return;
                if (submitBtn) submitBtn.disabled = true;
                if (moveError) moveError.style.display = 'none';

                api.moveFile(state.drop.current.id, file.id, dest.value).then(function (resp) {
                    if (submitBtn) submitBtn.disabled = false;
                    if (resp.success) {
                        moveDialog.close();
                        showToast('Moved to ' + (dest.value === '/' ? 'root' : dest.value), 'info');
                        loadFiles(state.drop.current.id, state.file.currentDir);
                    } else {
                        var msg = handleApiError(resp, 'Failed to move');
                        if (moveError) {
                            moveError.textContent = 'Error: ' + msg;
                            moveError.style.display = 'block';
                        }
                        showToast('Failed: ' + msg, 'error');
                    }
                });
            });
        }

        // Upload
        var uploadDialog = $('uploadDialog');
        var cancelUpload = $('cancelUpload');
        var uploadForm = $('uploadForm');
        var uploadError = $('uploadError');
        var uploadProgress = $('uploadProgress');

        if (cancelUpload && uploadDialog) {
            cancelUpload.addEventListener('click', function () { uploadDialog.close(); if (uploadError) uploadError.style.display = 'none'; });
        }
        if (uploadForm && uploadDialog) {
            uploadForm.addEventListener('submit', function (e) {
                e.preventDefault();
                var fileInput = $('uploadFileInput');
                var submitBtn = $('submitUpload');
                if (!fileInput || !fileInput.files || fileInput.files.length === 0 || !state.drop.current) return;
                if (submitBtn) submitBtn.disabled = true;
                if (uploadError) uploadError.style.display = 'none';
                if (uploadProgress) { uploadProgress.style.display = 'block'; uploadProgress.textContent = 'Uploading...'; }

                var files = Array.prototype.slice.call(fileInput.files);
                var completed = 0;
                var failed = 0;

                files.forEach(function (file) {
                    api.uploadFile(state.drop.current.id, file, state.file.currentDir).then(function (resp) {
                        completed++;
                        if (resp.success) {
                            if (uploadProgress) uploadProgress.textContent = 'Uploaded ' + completed + ' of ' + files.length;
                        } else {
                            failed++;
                            if (uploadProgress) uploadProgress.textContent = 'Uploaded ' + completed + ' of ' + files.length + ' (' + failed + ' failed)';
                        }
                        if (completed + failed === files.length) {
                            if (submitBtn) submitBtn.disabled = false;
                            if (uploadProgress) uploadProgress.style.display = 'none';
                            if (failed > 0) {
                                showToast(failed + ' file(s) failed to upload', 'error');
                            } else {
                                showToast('All files uploaded', 'info');
                            }
                            uploadDialog.close();
                            if (fileInput) fileInput.value = '';
                            loadFiles(state.drop.current.id, state.file.currentDir);
                        }
                    }).catch(function () {
                        completed++;
                        failed++;
                        if (completed + failed === files.length) {
                            if (submitBtn) submitBtn.disabled = false;
                            if (uploadProgress) uploadProgress.style.display = 'none';
                            showToast('Upload failed', 'error');
                            uploadDialog.close();
                            loadFiles(state.drop.current.id, state.file.currentDir);
                        }
                    });
                });
            });
        }

        // Share Drop
        var shareDialog = $('shareDropDialog');
        var cancelShare = $('cancelShareDrop');
        var shareForm = $('shareDropForm');
        var shareError = $('shareDropError');

        if (cancelShare && shareDialog) {
            cancelShare.addEventListener('click', function () { shareDialog.close(); if (shareError) shareError.style.display = 'none'; });
        }
        if (shareForm && shareDialog) {
            shareForm.addEventListener('submit', function (e) {
                e.preventDefault();
                var name = $('shareContactName');
                var perm = $('sharePermission');
                var submitBtn = $('submitShareDrop');
                if (!name || !name.value.trim() || !state.drop.current) return;
                if (submitBtn) submitBtn.disabled = true;
                if (shareError) shareError.style.display = 'none';

                api.shareDrop(state.drop.current.id, name.value.trim(), perm ? perm.value : 'viewer').then(function (resp) {
                    if (submitBtn) submitBtn.disabled = false;
                    if (resp.success) {
                        shareDialog.close();
                        if (name) name.value = '';
                        showToast('Drop shared with ' + name.value, 'info');
                    } else {
                        var msg = handleApiError(resp, 'Failed to share');
                        if (shareError) {
                            shareError.textContent = 'Error: ' + msg;
                            shareError.style.display = 'block';
                        }
                        showToast('Failed: ' + msg, 'error');
                    }
                });
            });
        }

        // Account Settings
        var settingsDialog = $('accountSettingsDialog');
        var cancelSettings = $('cancelAccountSettings');
        if (cancelSettings && settingsDialog) {
            cancelSettings.addEventListener('click', function () { settingsDialog.close(); });
        }

        // Profile menu
        var profileBtn = $('profileMenuButton');
        var profileMenu = $('profileMenu');
        if (profileBtn && profileMenu) {
            profileBtn.addEventListener('click', function (e) {
                e.stopPropagation();
                profileMenu.hidden = !profileMenu.hidden;
                profileBtn.setAttribute('aria-expanded', String(!profileMenu.hidden));
            });
            document.addEventListener('click', function (e) {
                if (!profileMenu.hidden && !e.target.closest('.profile-menu')) {
                    profileMenu.hidden = true;
                    profileBtn.setAttribute('aria-expanded', 'false');
                }
            });
        }

        // Open settings
        var openSettings = $('openAccountSettings');
        if (openSettings && settingsDialog) {
            openSettings.addEventListener('click', function () {
                if (profileMenu) profileMenu.hidden = true;
                settingsDialog.showModal();
            });
        }
    }

    // === Search ===
    function initSearch() {
        var searchInput = $('globalSearch');
        if (!searchInput) return;
        var searchResults = document.createElement('div');
        searchResults.className = 'v4-search-results';
        searchResults.id = 'searchResultsDropdown';
        var wrap = qs('.v4-search-wrap');
        if (wrap) wrap.appendChild(searchResults);

        searchInput.addEventListener('input', function () {
            clearTimeout(searchInput._timer);
            var q = searchInput.value.trim();
            if (q.length < 2) { searchResults.classList.remove('visible'); searchResults.innerHTML = ''; return; }
            searchInput._timer = setTimeout(function () {
                api.search(q).then(function (resp) {
                    if (!resp.success) return;
                    searchResults.innerHTML = '';
                    var hasAny = false;
                    ['drops', 'files', 'users'].forEach(function (type) {
                        var items = (resp.data && resp.data[type]) || [];
                        if (items.length === 0) return;
                        hasAny = true;
                        var section = document.createElement('div');
                        section.className = 'v4-search-section';
                        section.innerHTML = '<div class="v4-search-section-title">' + type.charAt(0).toUpperCase() + type.slice(1) + '</div>';
                        items.forEach(function (item) {
                            var el = document.createElement('div');
                            el.className = 'v4-search-item';
                            el.textContent = item.name + (item.drop_name ? ' — ' + item.drop_name : '');
                            el.addEventListener('click', function () {
                                searchResults.classList.remove('visible');
                                if (type === 'drops' && item.id) {
                                    api.getDrop(item.id).then(function (r) {
                                        if (r.success) openDrop(r.data);
                                    });
                                }
                            });
                            section.appendChild(el);
                        });
                        searchResults.appendChild(section);
                    });
                    if (!hasAny) {
                        searchResults.innerHTML = '<div class="v4-search-section"><div class="v4-search-item" style="color:var(--muted)">No results</div></div>';
                    }
                    searchResults.classList.add('visible');
                });
            }, 300);
        });
        searchInput.addEventListener('blur', function () { setTimeout(function () { searchResults.classList.remove('visible'); }, 200); });
        searchInput.addEventListener('focus', function () {
            if (searchResults.children.length > 0) searchResults.classList.add('visible');
        });
    }

    // === View Toggle ===
    function initViewToggle() {
        qsa('.v4-view-btn').forEach(function (btn) {
            btn.addEventListener('click', function () {
                var view = btn.getAttribute('data-view');
                if (!view) return;
                state.file.viewMode = view;
                qsa('.v4-view-btn').forEach(function (b) { b.classList.remove('active'); });
                btn.classList.add('active');
                if (state.ui.activeTab === 'home') renderDropBrowser();
                else if (state.ui.activeTab === 'drop') renderFilesView();
            });
        });
    }

    // === Keyboard Shortcuts ===
    function initKeyboard() {
        document.addEventListener('keydown', function (e) {
            // Ctrl+N: New Drop
            if (e.ctrlKey && e.key === 'n') {
                e.preventDefault();
                var d = $('createDropDialog');
                if (d) d.showModal();
            }
            // Ctrl+A: Select all (when in drop view)
            if (e.ctrlKey && e.key === 'a' && state.ui.activeTab === 'drop') {
                e.preventDefault();
                state.file.selected = state.file.currentFiles.map(function (f) { return f.id; });
                renderFilesView();
            }
            // Delete: Move selected to trash
            if (e.key === 'Delete' && state.ui.activeTab === 'drop' && state.file.selected.length > 0) {
                e.preventDefault();
                var selectedFiles = state.file.currentFiles.filter(function (f) {
                    return state.file.selected.indexOf(f.id) !== -1;
                });
                if (selectedFiles.length > 0) {
                    if (confirm('Move ' + selectedFiles.length + ' item(s) to Trash?')) {
                        var remaining = selectedFiles.length;
                        selectedFiles.forEach(function (f) {
                            api.trashFile(state.drop.current.id, f.id).then(function (resp) {
                                remaining--;
                                if (remaining === 0) {
                                    showToast('Moved to Trash', 'info');
                                    loadFiles(state.drop.current.id, state.file.currentDir);
                                }
                            });
                        });
                    }
                }
            }
            // Escape: Close dialogs and context menu
            if (e.key === 'Escape') {
                closeContextMenu();
                var dialogs = qsa('dialog[open]');
                dialogs.forEach(function (d) { d.close(); });
            }
        });
    }

    // === Utility ===
    function esc(str) {
        if (!str) return '';
        var d = document.createElement('div');
        d.textContent = str;
        return d.innerHTML;
    }

    function fmtSize(bytes) {
        if (!bytes || bytes === 0) return '0 B';
        var units = ['B', 'KB', 'MB', 'GB'];
        var i = Math.floor(Math.log(bytes) / Math.log(1024));
        return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i];
    }

    function fmtTime(unix) {
        if (!unix) return '';
        var d = new Date(unix * 1000);
        var now = new Date();
        var diff = now - d;
        if (diff < 60000) return 'Just now';
        if (diff < 3600000) return Math.floor(diff / 60000) + 'm ago';
        if (diff < 86400000) return Math.floor(diff / 3600000) + 'h ago';
        if (diff < 604800000) return Math.floor(diff / 86400000) + 'd ago';
        return d.toLocaleDateString();
    }

    function getIcon(name, type) {
        if (type === 'folder') {
            return '<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" fill="currentColor" opacity="0.3"/><path d="M20 6v12a2 2 0 01-2 2H6l-4-4V5a2 2 0 012-2h11l2 2h5a2 2 0 012 2v-1z" fill="currentColor"/></svg>';
        }
        if (type === 'document') {
            return '<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8l-6-6z" fill="none" stroke="currentColor" stroke-width="2"/><path d="M14 2v6h6M16 13H8M16 17H8M10 9H8" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>';
        }
        if (type === 'presentation') {
            return '<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><rect x="2" y="3" width="20" height="14" rx="2" fill="none" stroke="currentColor" stroke-width="2"/><path d="M8 21h8M12 17v4" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>';
        }
        if (!name) {
            return '<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8l-6-6z" fill="none" stroke="currentColor" stroke-width="2"/><path d="M14 2v6h6" fill="none" stroke="currentColor" stroke-width="2"/></svg>';
        }
        
        var ext = name.split('.').pop().toLowerCase();
        
        // Code files
        if (['html', 'htm', 'css', 'scss', 'less', 'js', 'ts', 'go', 'py', 'java', 'jar', 'rs', 'php', 'rb'].indexOf(ext) !== -1) {
            return '<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8l-6-6z" fill="none" stroke="currentColor" stroke-width="2"/><path d="M14 2v6h6M8 13l-2 8 4-2 4 2-2-8" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>';
        }
        
        // Data files
        if (['json', 'xml', 'yaml', 'yml', 'toml'].indexOf(ext) !== -1) {
            return '<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8l-6-6z" fill="none" stroke="currentColor" stroke-width="2"/><path d="M14 2v6h6M8 13l-2 8 4-2 4 2-2-8" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>';
        }
        
        // Images
        if (['png', 'jpg', 'jpeg', 'gif', 'svg', 'webp', 'bmp', 'ico'].indexOf(ext) !== -1) {
            return '<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><rect x="3" y="3" width="18" height="18" rx="2" fill="none" stroke="currentColor" stroke-width="2"/><circle cx="8.5" cy="8.5" r="1.5" fill="currentColor"/><path d="M21 15l-5-5L3 21" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>';
        }
        
        // Archives
        if (['zip', 'tar', 'gz', 'rar', '7z'].indexOf(ext) !== -1) {
            return '<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8l-6-6z" fill="none" stroke="currentColor" stroke-width="2"/><path d="M14 2v6h6M10 14l-2 6 2-1 2 1-2-6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>';
        }
        
        // Audio
        if (['mp3', 'wav', 'ogg', 'flac', 'm4a'].indexOf(ext) !== -1) {
            return '<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M9 18V5l12-2v13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/><circle cx="6" cy="18" r="3" fill="none" stroke="currentColor" stroke-width="2"/><circle cx="18" cy="16" r="3" fill="none" stroke="currentColor" stroke-width="2"/></svg>';
        }
        
        // Video
        if (['mp4', 'mov', 'avi', 'mkv', 'webm'].indexOf(ext) !== -1) {
            return '<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><rect x="2" y="4" width="20" height="16" rx="2" fill="none" stroke="currentColor" stroke-width="2"/><path d="M10 9l5 3-5 3V9z" fill="currentColor"/></svg>';
        }
        
        // Markdown
        if (ext === 'md' || ext === 'markdown') {
            return '<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8l-6-6z" fill="none" stroke="currentColor" stroke-width="2"/><path d="M14 2v6h6M7 15l2 2 4-4" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>';
        }
        
        // Text
        if (ext === 'txt' || ext === 'text') {
            return '<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8l-6-6z" fill="none" stroke="currentColor" stroke-width="2"/><path d="M14 2v6h6M16 13H8M16 17H8M10 9H8" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>';
        }
        
        // Documents
        if (['pdf', 'doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx'].indexOf(ext) !== -1) {
            return '<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8l-6-6z" fill="none" stroke="currentColor" stroke-width="2"/><path d="M14 2v6h6" fill="none" stroke="currentColor" stroke-width="2"/></svg>';
        }
        
        // Default file icon
        return '<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8l-6-6z" fill="none" stroke="currentColor" stroke-width="2"/><path d="M14 2v6h6" fill="none" stroke="currentColor" stroke-width="2"/></svg>';
    }

    // === Init ===
    function init() {
        initTheme();
        initDialogs();
        initActions();
        initSearch();
        initViewToggle();
        initKeyboard();

        // Tab switching
        qsa('.v4-nav-link[data-tab]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                switchTab(btn.getAttribute('data-tab'));
            });
        });

        // Start on home tab (drop browser)
        switchTab('home');
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();