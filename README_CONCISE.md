# FtR Project — Concise Overview

**FtR** (File Transfer & Repository Management) is a unified, open-source ecosystem for managing software packages, distributing files, and enabling real-time collaborative editing. It combines a powerful CLI tool with an InkDrop backend server to solve fragmentation in modern software distribution and collaborative development.

## The Problem

Modern software development suffers from:
- **Language fragmentation**: Separate tools for Python (pip), Node (npm), Go (go get), C++ (manual)
- **Build complexity**: CI/CD pipelines for multi-language projects are tedious
- **Collaboration friction**: Teams lack efficient, real-time file editing like Google Docs
- **Network unreliability**: Large uploads fail frequently without resumable support
- **Version chaos**: Developers don't know which packages are installed or outdated

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
POST  /new/file, /new/dir, /rename
DELETE /delete/item, /trash/empty
GET   /download                       # File/directory download
POST  /api/fs/*                       # Filesystem API
GET   /api/contacts/list              # Contact management
GET   /api/profile, /api/user/profile # User profiles
```

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

- **Client-side encryption**: AES-256-CBC for sensitive files
- **Password hashing**: bcrypt with configurable cost factor (16)
- **Session security**: Secure cookie handling, PHPSESSID validation
- **Input validation**: Regex validation (alphanumeric, hyphen, underscore only)
- **Authorization checks**: Per-repository ownership verification
- **Trash-first deletion**: Files moved to `.Trash-1000` before permanent removal
- **Profile pictures**: Auto-cropped, resized (200×200), JPEG compressed
- **Profile bio**: 500-character limit, XSS-safe JSON escaping

## Multi-Language Build Support (Priority Order)

1. Pre-built binaries in `BUILD/linux-{arch}/`
2. Windows MSI in `BUILD/windows/`
3. Custom `BUILD_COMMAND` from metadata
4. Custom `INSTALL_COMMAND` from metadata
5. `install.sh` shell scripts
6. Pre-built ELF binaries
7. `Makefile`
8. Python (`main.py`) via PyInstaller
9. Go (`main.go`) via go build
10. C++ (`main.cpp`) via g++
11. SQU1D (`main.sqd`) via squ1d++

## User Features

### Repository Management
- Create, delete, rename, move files and directories
- Download single files or entire repositories as ZIP
- Share repositories with other users (owner model)
- Trash recovery for accidentally deleted files
- Repository visibility (public/private)

### Collaborative Editing
- Real-time editing with multiple users
- Presence awareness (who's currently editing)
- Syntax highlighting for 20+ file types
- 5MB live edit limit; larger files use document editor
- SSE-based live updates with conflict resolution

### Profile & Contacts System
- User bio (max 500 characters)
- Auto-cropped, resized profile pictures
- Add/remove contacts from other users
- Contact-aware owner dropdown when sharing repositories
- User search with filtering
- Public user profiles with bio and picture

### FUSE Mounting
- Mount remote repositories as local directories
- On-demand file downloading (placeholder until accessed)
- Automatic upload of changes back to repository
- Desktop file manager integration with notifications
- Read-only and read-write modes
- Parallel uploads/downloads (6 workers)

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
| **Frontend** | HTML/CSS/JS | - |
| **Editor** | Ace (embedded) | - |

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

# Share with contacts
# (Use InkDrop web UI: Settings → Contacts → Add)

# Download and build
ftr get myusername/myrepo@1.0.0

# Mount for file manager access
ftr mount myusername/myrepo ~/mnt

# Auto-mount on boot
ftr automount install myusername/myrepo ~/mnt
ftr automount enable myusername/myrepo
```

## License

**MIT License with Commons Clause** — Use and modify freely, but cannot commercially redistribute the software itself.

---

*FtR Project © 2026 Quan Thai. Unified software distribution, collaborative development, and file repository management in one ecosystem.*
