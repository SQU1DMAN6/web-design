/**
 * InkDrop 4.0 — Client-Side Application
 * Complete rewrite with functional drop management, contacts, and sharing
 */
(function () {
    'use strict';

    var CONFIG = window.__INKDROP_INIT__ || {
        userName: 'Guest',
        userPFP: '/pfp/default.png',
        userBio: '',
        currentDropID: '',
        currentDropName: '',
        apiBase: '/api/v4'
    };

    var state = {
        activeTab: 'home',
        drops: [],
        allDrops: [],
        sharedDrops: [],
        publicDrops: [],
        contacts: [],
        currentDrop: null,
        currentFiles: [],
        viewMode: 'grid',
        searchQuery: '',
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
        del: function (path) {
            return fetch(this.base + path, { method: 'DELETE', credentials: 'same-origin' }).then(function (r) { return r.json(); });
        },
        listDrops: function () { return this.get('/drops'); },
        getDrop: function (id) { return this.get('/drop/' + id); },
        createDrop: function (name, desc) { return this.post('/drops', { name: name, description: desc || '' }); },
        deleteDrop: function (id) { return this.del('/drop/' + id); },
        listFiles: function (id) { return this.get('/drop/' + id + '/files'); },
        getFileContent: function (did, fid) { return this.get('/drop/' + did + '/files/' + fid + '/content'); },
        shareDrop: function (id, userName, perm) { return this.post('/drop/' + id + '/share', { user_name: userName, permission: perm || 'viewer' }); },
        getActivity: function (id) { return this.get('/drop/' + id + '/activity'); },
        search: function (q) { return this.get('/search?q=' + encodeURIComponent(q)); },
        getContacts: function () { return fetch('/api/contacts', { credentials: 'same-origin' }).then(function (r) { return r.json(); }); },
        sendContactRequest: function (name) {
            return fetch('/contacts/request', {
                method: 'POST', credentials: 'same-origin',
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                body: 'recipient=' + encodeURIComponent(name)
            }).then(function (r) { return r.json(); });
        },
        respondContact: function (id, action) {
            return fetch('/contacts/respond', {
                method: 'POST', credentials: 'same-origin',
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                body: 'id=' + encodeURIComponent(id) + '&action=' + encodeURIComponent(action)
            }).then(function (r) { return r.json(); });
        },
        removeContact: function (name) {
            return fetch('/contacts/remove', {
                method: 'POST', credentials: 'same-origin',
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                body: 'contact=' + encodeURIComponent(name)
            }).then(function (r) { return r.json(); });
        },
        searchUsers: function (q) {
            return fetch('/api/community?q=' + encodeURIComponent(q), { credentials: 'same-origin' }).then(function (r) { return r.json(); });
        }
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

    // === Theme ===
    function initTheme() {
        var key = 'inkdrop-theme';
        function setTheme(v) {
            document.documentElement.setAttribute('data-theme', v);
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
        state.activeTab = tab;
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
                    (resp.error ? resp.error.message : 'Authentication error. Please try logging in again.') +
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

        var viewMode = state.viewMode;

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
                    '<div class="v4-file-list-icon">📁</div>' +
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
        state.currentDrop = drop;
        state.activeTab = 'drop';

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
        loadFiles(drop.id);
    }

    function loadFiles(dropID) {
        var content = $('workspaceContent');
        if (!content) return;
        content.innerHTML = '<div class="v4-workspace-empty"><p>Loading files...</p></div>';

        api.listFiles(dropID).then(function (resp) {
            if (resp.success) {
                state.currentFiles = resp.data || [];
                renderFilesView();
                updateProjectTree();
            } else {
                content.innerHTML = '<div class="v4-workspace-empty"><h3>Could not load files</h3></div>';
            }
        });
    }

    function renderFilesView() {
        var content = $('workspaceContent');
        if (!content) return;
        var files = state.currentFiles || [];

        if (files.length === 0) {
            content.innerHTML = '' +
                '<div class="v4-workspace-empty">' +
                '<h3>This Drop is empty</h3>' +
                '<p>Upload files using the Actions panel, or create new documents.</p>' +
                '</div>';
            return;
        }

        var viewMode = state.viewMode;
        if (viewMode === 'grid') {
            var grid = document.createElement('div');
            grid.className = 'v4-file-grid';
            files.forEach(function (f) {
                var card = document.createElement('div');
                card.className = 'v4-file-card';
                card.innerHTML = '' +
                    '<div class="v4-file-card-icon">' + getIcon(f.name, f.type) + '</div>' +
                    '<div class="v4-file-card-name">' + esc(f.name) + '</div>' +
                    '<div class="v4-file-card-size">' + fmtSize(f.size) + '</div>';
                card.addEventListener('click', function () { previewFile(f); });
                grid.appendChild(card);
            });
            content.innerHTML = '';
            content.appendChild(grid);
        } else {
            var list = document.createElement('div');
            list.className = 'v4-file-list';
            files.forEach(function (f) {
                var item = document.createElement('div');
                item.className = 'v4-file-list-item';
                item.innerHTML = '' +
                    '<div class="v4-file-list-icon">' + getIcon(f.name, f.type) + '</div>' +
                    '<div class="v4-file-list-name">' + esc(f.name) + '</div>' +
                    '<div class="v4-file-list-size">' + fmtSize(f.size) + '</div>' +
                    '<div class="v4-file-list-date">' + fmtTime(f.updated_at) + '</div>';
                item.addEventListener('click', function () { previewFile(f); });
                list.appendChild(item);
            });
            content.innerHTML = '';
            content.appendChild(list);
        }
    }

    function previewFile(file) {
        if (!state.currentDrop) return;
        showToast('Opening ' + file.name + '...', 'info');
        api.getFileContent(state.currentDrop.id, file.id).then(function (data) {
            var text = typeof data === 'string' ? data : JSON.stringify(data, null, 2);
            if (text && text.length > 500) text = text.substring(0, 500) + '\n\n... (file truncated)';
            // Show in a simple viewer
            var content = $('workspaceContent');
            if (!content) return;
            content.innerHTML = '' +
                '<div class="v4-file-preview">' +
                '<div class="v4-file-preview-header">' +
                '<span class="v4-file-preview-title">' + esc(file.name) + '</span>' +
                '<button type="button" class="button-link small" id="closePreview">Back to Files</button>' +
                '</div>' +
                '<pre class="v4-file-preview-content">' + esc(text || 'No content') + '</pre>' +
                '</div>';
            var closeBtn = $('closePreview');
            if (closeBtn) closeBtn.addEventListener('click', function () { renderFilesView(); });
        }).catch(function () {
            showToast('Could not read file content', 'error');
        });
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
            '<button type="button" class="v4-drop-action-btn" id="activityBtn"><svg viewBox="0 0 24 24" width="16" height="16"><circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" fill="none"/><path d="M12 6v6l4 2" stroke="currentColor" stroke-width="2" fill="none"/></svg> Recent Activity</button>' +
            '</div></div>';

        var shareBtn = $('shareDropBtn');
        if (shareBtn) shareBtn.addEventListener('click', function () { showShareDialog(drop); });
        var actBtn = $('activityBtn');
        if (actBtn) actBtn.addEventListener('click', function () { showActivity(drop.id); });
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
            if (resp.success && resp.contacts && suggestions) {
                suggestions.innerHTML = '';
                resp.contacts.forEach(function (c) {
                    var opt = document.createElement('option');
                    opt.value = c;
                    suggestions.appendChild(opt);
                });
            }
        });

        if (contactInput) contactInput.value = '';
        var errorEl = $('shareDropError');
        if (errorEl) errorEl.style.display = 'none';
        dialog.showModal();
    }

    // === Shared/Public Views ===
    function renderSharedView() {
        var content = $('workspaceContent');
        if (!content) return;
        content.innerHTML = '<div class="v4-workspace-empty"><p>Loading shared drops...</p></div>';

        fetch('/?view=shared', { credentials: 'same-origin' }).then(function (r) { return r.text(); }).then(function (html) {
            // Parse the drop list from the server-rendered page
            // For simplicity, just show a message
            content.innerHTML = '' +
                '<div class="v4-workspace-empty">' +
                '<h3>Shared With Me</h3>' +
                '<p>Drops that others have shared with you will appear here.</p>' +
                '<p class="subtle">Use the Share button on any Drop to invite collaborators.</p>' +
                '</div>';
        }).catch(function () {
            content.innerHTML = '<div class="v4-workspace-empty"><h3>Could not load</h3></div>';
        });
    }

    function renderPublicView() {
        var content = $('workspaceContent');
        if (!content) return;
        content.innerHTML = '' +
            '<div class="v4-workspace-empty">' +
            '<h3>Public Drops</h3>' +
            '<p>Community drops will be shown here in a future update.</p>' +
            '</div>';
    }

    // === Contacts View ===
    function renderContactsView() {
        var content = $('workspaceContent');
        if (!content) return;
        content.innerHTML = '<div class="v4-workspace-empty"><p>Loading contacts...</p></div>';

        api.getContacts().then(function (resp) {
            var contacts = (resp.success && resp.contacts) ? resp.contacts : [];
            // Also fetch pending requests
            fetch('/api/community?q=', { credentials: 'same-origin' })
                .then(function (r) { return r.json(); })
                .then(function (community) {
                    var users = (community.success && community.users) ? community.users : [];
                    renderContactsPage(content, contacts, users);
                }).catch(function () {
                    renderContactsPage(content, contacts, []);
                });
        }).catch(function () {
            content.innerHTML = '<div class="v4-workspace-empty"><h3>Could not load contacts</h3></div>';
        });
    }

    function renderContactsPage(container, contacts, users) {
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
                contacts.forEach(function (name) {
                    var card = document.createElement('div');
                    card.className = 'v4-contact-card';
                    card.innerHTML = '' +
                        '<div class="v4-contact-avatar">' +
                        '<svg viewBox="0 0 24 24" width="20" height="20" fill="var(--muted)"><circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0116 0"/></svg>' +
                        '</div>' +
                        '<div class="v4-contact-name">' + esc(name) + '</div>' +
                        '<button type="button" class="button-link small danger" data-remove="' + esc(name) + '">Remove</button>';
                    var removeBtn = card.querySelector('[data-remove]');
                    if (removeBtn) {
                        removeBtn.addEventListener('click', function () {
                            api.removeContact(name).then(function (resp) {
                                if (resp.success) { showToast('Contact removed', 'info'); renderContactsView(); }
                                else { showToast('Could not remove contact', 'error'); }
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
                if (!resp.success || !resp.users || resp.users.length === 0) {
                    resultsEl.innerHTML = '<div class="subtle">No users found.</div>';
                    return;
                }
                resultsEl.innerHTML = '';
                resp.users.forEach(function (user) {
                    var card = document.createElement('div');
                    card.className = 'v4-contact-card';
                    var isContact = contacts.indexOf(user.name) !== -1;
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
                            api.sendContactRequest(user.name).then(function (r) {
                                if (r.success) { showToast('Request sent to ' + user.name, 'info'); doSearch(); }
                                else { showToast('Could not send request', 'error'); addBtn.disabled = false; addBtn.textContent = 'Add'; }
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
        var files = state.currentFiles || [];
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
                item.addEventListener('click', function (e) { e.preventDefault(); previewFile(f); });
                container.appendChild(item);
            });
        }

        renderTree(fileTree, files.filter(function (f) { return f.type === 'file' || !f.type; }), '📄');
        renderTree(docTree, files.filter(function (f) { return f.type === 'document'; }), '📝');
        renderTree(trashTree, [], '🗑️');
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

        // Upload (placeholder - opens file dialog / shows info)
        qsa('[data-action="upload"]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                if (state.currentDrop) {
                    showToast('Upload via Inker or FtR tools (CLI)', 'info');
                } else {
                    showToast('Select a Drop first', 'warning');
                }
            });
        });

        // New Folder (placeholder)
        qsa('[data-action="new-folder"]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                if (state.currentDrop) {
                    showToast('Folder creation coming soon', 'info');
                } else {
                    showToast('Select a Drop first', 'warning');
                }
            });
        });

        // Delete Drop
        qsa('[data-action="delete-drop"]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                if (!state.currentDrop) {
                    showToast('Select a Drop first', 'warning');
                    return;
                }
                if (confirm('Delete "' + state.currentDrop.name + '"? This cannot be undone.')) {
                    api.deleteDrop(state.currentDrop.id).then(function (resp) {
                        if (resp.success) {
                            showToast('Drop deleted', 'info');
                            state.currentDrop = null;
                            state.currentFiles = [];
                            var panel = $('dropPanelContent');
                            if (panel) panel.innerHTML = '<div class="v4-drop-panel-placeholder"><svg viewBox="0 0 24 24" width="32" height="32" stroke="var(--muted)" stroke-width="1.5" fill="none"><path d="M6 3h9l4 4v14H6z"/><path d="M15 3v5h5"/></svg><p>Select a Drop<br/>to view details</p></div>';
                            switchTab('home');
                        } else {
                            showToast('Could not delete: ' + (resp.error ? resp.error.message : 'Unknown error'), 'error');
                        }
                    });
                }
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
                        var msg = resp.error ? resp.error.message : 'Unknown error';
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
                if (!name || !name.value.trim() || !state.currentDrop) return;
                if (submitBtn) submitBtn.disabled = true;
                if (shareError) shareError.style.display = 'none';

                api.shareDrop(state.currentDrop.id, name.value.trim(), perm ? perm.value : 'viewer').then(function (resp) {
                    if (submitBtn) submitBtn.disabled = false;
                    if (resp.success) {
                        shareDialog.close();
                        if (name) name.value = '';
                        showToast('Drop shared with ' + name.value, 'info');
                    } else {
                        var msg = resp.error ? resp.error.message : 'Unknown error';
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
                state.viewMode = view;
                qsa('.v4-view-btn').forEach(function (b) { b.classList.remove('active'); });
                btn.classList.add('active');
                if (state.activeTab === 'home') renderDropBrowser();
                else if (state.activeTab === 'drop') renderFilesView();
            });
        });
    }

    // === Keyboard Shortcuts ===
    function initKeyboard() {
        document.addEventListener('keydown', function (e) {
            if (e.ctrlKey && e.key === 'n') {
                e.preventDefault();
                var d = $('createDropDialog');
                if (d) d.showModal();
            }
            if (e.key === 'Escape') {
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
        if (type === 'folder') return '📁';
        if (type === 'document') return '📝';
        if (type === 'presentation') return '🎞️';
        if (!name) return '📄';
        var ext = name.split('.').pop().toLowerCase();
        var icons = {
            txt: '📄', md: '📝', html: '🌐', css: '🎨', js: '⚡',
            go: '🔵', py: '🐍', java: '☕', rs: '🦀',
            json: '📋', xml: '📋', yaml: '📋', yml: '📋',
            png: '🖼️', jpg: '🖼️', jpeg: '🖼️', gif: '🖼️', svg: '🖼️', webp: '🖼️',
            pdf: '📕', doc: '📘', docx: '📘', xls: '📗', xlsx: '📗',
            zip: '📦', tar: '📦', gz: '📦', rar: '📦',
            mp3: '🎵', wav: '🎵', ogg: '🎵',
            mp4: '🎬', mov: '🎬', avi: '🎬',
        };
        return icons[ext] || '📄';
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