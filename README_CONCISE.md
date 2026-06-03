# FtR Project — Concise Overview

**FtR** (File Transfer & Repository Management) is a unified, open-source ecosystem for managing software packages, distributing files, and enabling real-time collaborative editing. It combines a powerful CLI tool with an InkDrop backend server to solve fragmentation in modern software distribution and collaborative development.

**Version:** 3.0  
**License:** MIT with Commons Clause | **Copyright:** © 2026 Quan Thai

## The Problem

Modern software development suffers from:
1. **Language fragmentation**: Separate tools for Python (pip), Node (npm), Go (go get), C++ (manual)
2. **Build complexity**: CI/CD pipelines for multi-language projects are tedious
3. **Collaboration friction**: Teams lack efficient, real-time file editing like Google Docs
4. **Network unreliability**: Large uploads fail frequently without resumable support
5. **Security concerns**: Sensitive files transmitted plaintext over networks
6. **Version chaos**: Developers don't know which packages are installed or outdated

## The Solution: Two-Tier Architecture

### **Tier 1: FtR CLI** (Go, Cobra framework)
A unified command-line package manager that:
- Downloads, installs, and manages packages from Inkdrop repositories
- **Auto-detects project types** (Go, Python, C++, Makefile, shell scripts, SQU1D) and compiles them
- **Encrypts files client-side** with AES-256-CBC before uploading
- Performs **resumable uploads** via TUS protocol for reliability
- Manages a **local registry** (`~/.local/share/ftr/registry.json`) tracking installed packages
- Authenticates via session-based login with password hashing (bcrypt)

**Key Commands:**
```bash
ftr get user/repo@1.2.3          # Download & auto-build
ftr up file.zip user/repo -E     # Upload with encryption
ftr pack directory               # Create SQAR/FSDL archives
ftr mount user/repo ~/mnt        # Mount as FUSE directory
ftr automount install user/repo  # Systemd auto-mount service
ftr search query                 # Search repositories
```

### **Tier 2: InkDrop** (Go, Chi Router, SQLite, FUSE)
A web backend server providing:
- **REST API** for file operations, authentication, and package discovery
- **Real-time collaborative editing** via Server-Sent Events (SSE) with presence awareness
- **FUSE mounts** for seamless desktop file manager integration
- **Resumable uploads** (TUS protocol) and bulk downloads
- **Fine-grained permissions** (public/private, multi-owner repositories)
- **Profile & contact system** for user networking and repository sharing
- Secure **session management** and authentication

**Core Endpoints:**
```
POST  /register, /login, /logout
GET   /browse/{user}/{repo}/*        # File browser
GET   /edit/{file}/{user}/{repo}/*   # Live editor
POST  /new/file, /new/dir, /rename, /delete
GET   /download                      # File/directory download
POST  /api/fs/*                      # Filesystem API
GET   /api/contacts/list             # Contact management
GET   /api/profile, /api/user/profile # User profiles
POST  /upload                        # TUS resumable upload
```

## Core Workflows

### Workflow 1: Share a Multi-Language Project

**Scenario:** You maintain a project in Go + Python + C++.

```bash
# Step 1: Create project structure
my-project/
├── main.go           # Go component
├── script.py         # Python component
├── utils.cpp         # C++ component
├── Makefile
└── install.sh

# Step 2: Upload to FtR
ftr pack ./my-project
ftr up ./my-project.sqar myuser/myproject -E   # -E encrypts

# Step 3: Your users just run
ftr get myuser/myproject@1.2.3

# FtR automatically:
# ✓ Downloads the archive
# ✓ Decrypts if necessary
# ✓ Detects Go, Python, C++ components
# ✓ Compiles each with appropriate toolchain
# ✓ Places binaries in ~/.local/ftr/
# ✓ Registers in local package registry
```

**Why this is better:** Users don't need build tools. Cross-platform automatic. No documentation to maintain. Integrity guaranteed with SHA256.

### Workflow 2: Real-Time Team Editing

**Scenario:** Your team needs to update deployment configs together.

```bash
# Step 1: Upload config
ftr up deployment-config.yaml myteam/ops

# Step 2: Team opens in browser
# http://localhost:6767/edit/deployment-config.yaml/myteam/ops

# Everyone sees:
# ✓ Live updates (changes appear instantly)
# ✓ Who's editing (multiple cursors with names)
# ✓ Syntax highlighting (YAML, JSON, code)
# ✓ Version history (all changes tracked)
# ✓ Auto-save (every 30 seconds)

# Step 3: Download final version
ftr down myteam/ops
```

**Why this is better:** Faster than email or Git commits. See changes as they happen. Built-in audit trail.

### Workflow 3: Secure Binary Distribution

**Scenario:** Distribute proprietary tools to 100 developers securely.

```bash
# Step 1: Build and encrypt
ftr pack ./secure-tool
ftr up ./secure-tool.sqar company/tools -E   # AES-256 encrypt

# Step 2: Developers download (decryption automatic)
ftr get company/tools

# Step 3: Distribute new version
ftr up ./new-version.sqar company/tools -E
ftr list --upgradeable
ftr upgrade company/tools
```

**Why this is better:** Files encrypted in transit (server never sees plaintext). Developers hold keys. Automatic version tracking. SHA256 integrity guaranteed.

### Workflow 4: Mount Repos in File Manager

**Scenario:** Your team wants to work with files using Finder/Explorer.

```bash
# Step 1: Mount the repository
ftr mount myuser/myrepo ~/mnt/myrepo

# Step 2: Use file manager normally
# - Drag files in → auto-uploaded
# - Edit files locally → synced to server
# - Copy/paste works normally
# - Use VSCode, Sublime on mounted files

# Step 3: Auto-mount on system startup
ftr automount install myuser/myrepo ~/mnt/myrepo
ftr automount enable myuser/myrepo
```

**Why this is better:** No learning curve. Works like Dropbox/OneDrive for code. Desktop notifications on changes.

## Storage Architecture

```
Client-side (~/.config/ftr/):
├── session            # Session ID
├── email, username    # Logged-in user
└── keys/              # Per-file encryption keys

Server-side (/srv/ftr/):
├── userRepositories/{user}/{repo}/   # File storage
├── _meta/{user}/{repo}/               # Metadata (owners, permissions)
│   └── pfp/                           # Profile pictures
├── .Trash-1000/                       # Deleted files (safety-first delete)
└── tmp/uploads/, archives/            # Temporary storage
```

## Security & Integrity

- **Client-side encryption**: AES-256-CBC for sensitive files (server never sees plaintext)
- **Password hashing**: bcrypt with configurable cost factor (16)
- **Session security**: Secure cookie handling, PHPSESSID validation
- **Input validation**: Regex validation (alphanumeric, hyphen, underscore only)
- **Authorization checks**: Per-repository ownership verification
- **Trash-first deletion**: Files moved to `.Trash-1000` before permanent removal
- **Profile pictures**: Auto-cropped, resized (200×200), JPEG compressed
- **Profile bio**: 500-character limit, XSS-safe JSON escaping
- **SHA256 integrity**: All uploads/downloads verified for tampering
- **Multi-owner model**: Fine-grained access control per repository

## Multi-Language Build Support

FtR automatically detects and builds projects (priority order):
1. Pre-built binaries in `BUILD/linux-{arch}/` (fastest)
2. Windows MSI in `BUILD/windows/`
3. Custom `BUILD_COMMAND` from metadata
4. Custom `INSTALL_COMMAND` from metadata
5. `install.sh` shell scripts
6. Pre-built ELF binaries
7. `Makefile`
8. Python (`main.py`) via PyInstaller
9. Go (`main.go`) via go build
10. C++ (`main.cpp`) via g++
11. SQU1D (`main.sqd`) via squ1dcc

**Supported Languages:** Go, Python, C++, Makefile-based, shell scripts, SQU1D

## Key Features

### FtR CLI Commands
```bash
ftr get user/repo@1.2.3          # Download & auto-build
ftr up file.zip user/repo -E     # Upload with AES-256 encryption
ftr pack directory               # Create SQAR/FSDL archives
ftr down user/repo               # Bulk download
ftr search query                 # Search repositories
ftr list --upgradeable           # Check for updates
ftr upgrade user/repo            # Upgrade packages
ftr mount user/repo ~/mnt        # Mount as FUSE directory
ftr automount install user/repo  # Systemd auto-mount service
ftr login / logout               # Session management
```

### InkDrop Web UI
- **Browse files** with syntax highlighting and previews
- **Upload files** with drag-and-drop (resumable via TUS)
- **Real-time collaborative editing** with presence awareness
- **Live version history** for all changes
- **Profile management** (picture, bio, contacts)
- **Contact system** for trusted collaborators
- **Repository sharing** with fine-grained permissions
- **Trash recovery** for accidentally deleted files

### Collaborative Features
- **Real-time editing**: Multiple users edit simultaneously
- **Presence awareness**: See who's currently editing
- **Syntax highlighting**: 20+ supported file types
- **Auto-save**: Every 30 seconds
- **Conflict resolution**: Automatic merge of concurrent edits
- **5MB live edit limit**: Larger files use document editor
- **Full version history**: Track all changes

## Technology Stack

| Component | Technology | Version |
|-----------|-----------|---------|
| **FtR CLI** | Go | 1.24.8 |
| **InkDrop Backend** | Go | 1.25.6 |
| **CLI Framework** | Cobra | v1+ |
| **HTTP Router** | Chi Router | v5 |
| **Database** | SQLite | - |
| **ORM** | bun | - |
| **FUSE** | bazil.org/fuse | - |
| **Upload Protocol** | TUS | - |
| **Encryption** | AES-256-CBC | - |
| **Password Hashing** | bcrypt | - |
| **Frontend** | HTML/CSS/JavaScript | - |
| **Editor** | Ace (embedded) | - |

## Quick Data Flows

**Download Flow (ftr get):**
```
User → Authenticate → Query API → Download Package → Detect Type → Build → Install
```

**Upload Flow (ftr up):**
```
File → Chunk → Parallel Upload (TUS) → Verify SHA256 → Register
```

**Collaboration Flow (Live Edit):**
```
User A Opens Editor → Server Streams SSE → User B Opens Same File
User A Edits → Change Delta Sent → Server Broadcasts → User B Receives Update
Changes Auto-Saved → Version History Maintained
```

## Installation & Quick Start

### FtR CLI
```bash
cd ftr
go build -o bin/ftr
sudo install -m 755 bin/ftr /usr/local/bin/ftr
```

### InkDrop Backend
```bash
cd inkdrop
go build -o bin/inkdrop
./bin/inkdrop  # Runs on http://localhost:6767
```

### First Steps
```bash
# Register and login
ftr login

# Create a repository
ftr up myfile.zip myusername/myrepo

# Share with contacts (use web UI)
# http://localhost:6767 → Settings → Contacts → Add

# Download and build (auto-detects everything)
ftr get myusername/myrepo@1.0.0

# Mount for file manager access
ftr mount myusername/myrepo ~/mnt

# Auto-mount on boot
ftr automount install myusername/myrepo ~/mnt
ftr automount enable myusername/myrepo
```

## Use Cases

| Use Case | Solution |
|----------|----------|
| Distributing multi-language projects | `ftr pack` + `ftr get` (auto-builds) |
| Team config management | Real-time editor + SSE sync |
| Proprietary software | Encrypted upload (`-E` flag) |
| Desktop file sync | FUSE mounting (`ftr mount`) |
| CI/CD pipelines | Automated `ftr` commands in GitHub Actions |
| Local package registry | `~/.local/share/ftr/registry.json` |
| Secure collaboration | Profile system + contacts + permissions |

## Common Commands Reference

```bash
# Package Management
ftr get user/repo@1.2.3        # Download specific version
ftr get user/repo -A            # Interactive file selection
ftr list --upgradeable          # Check for updates
ftr upgrade user/repo           # Upgrade package

# Uploads & Sharing
ftr up myapp.zip user/repo           # Plain upload
ftr down user/repo                   # Bulk download

# Repository Operations
ftr search "database"                # Find packages
ftr query user/repo                  # Get info

# FUSE Mounting
ftr mount user/repo ~/mnt            # Mount repository
ftr automount install user/repo ~/m  # Create service
ftr automount enable user/repo       # Enable auto-mount

# Session Management
ftr login                      # Authenticate
ftr logout                     # Clear session
ftr session                    # Show current session

# Local Management
ftr list                       # Installed packages
ftr remove user/repo           # Uninstall
```

## License

**MIT License with Commons Clause** — Use and modify freely, but cannot commercially redistribute the software itself.

---

*FtR Project © 2026 Quan Thai. Unified software distribution, collaborative development, and file repository management in one ecosystem.*
