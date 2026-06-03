# The FtR Project — Usage-Focused Guide

A comprehensive ecosystem for managing, distributing, and collaboratively editing software packages and file repositories.

**Version:** 3.0  
**Author:** Quan Thai  
**License:** MIT License with Commons Clause condition

---

## Table of Contents

1. [What is FtR?](#what-is-ftr)
2. [Quick Start (30 seconds)](#quick-start-30-seconds)
3. [Core Workflows](#core-workflows)
4. [Feature Guide](#feature-guide)
5. [Installation](#installation)
6. [Usage Examples](#usage-examples)
7. [Technology Stack](#technology-stack)
8. [License](#license)

---

## What is FtR?

**FtR** is a unified solution to three major problems in software development:

### Problem 1: Package Distribution Fragmentation
Traditional software distribution requires different tools for each language:
- Python: `pip`
- Node.js: `npm`
- Go: `go get`
- C++: Manual downloads
- Rust: `cargo`

**FtR solves this:** One command works everywhere. `ftr get myuser/myrepo` automatically detects your platform, architecture, and project type—then downloads and builds it correctly.

### Problem 2: Team Collaboration Friction
Distributed teams struggle to edit shared files efficiently:
- Email attachments get out of date
- Git commits feel slow for quick changes
- Wikis and docs fall behind
- Configuration files are hard to review together

**FtR solves this:** Real-time collaborative editing (like Google Docs) for any file type. Multiple people can edit the same file, see each other's changes instantly, and maintain full version history.

### Problem 3: Secure Distribution
Sensitive files need encryption and integrity verification:
- Plain uploads are unsafe
- Large files fail mid-transfer with no recovery
- Developers don't know which versions are installed
- Updates are manual

**FtR solves this:** Client-side AES-256 encryption, resumable uploads, automatic build detection, and local package registry.

---

## Quick Start (30 seconds)

### Download & Install FtR CLI

```bash
git clone https://github.com/your-org/ftr-project
cd ftr-project/ftr
./install.sh
```

### Start InkDrop Server

```bash
cd ../inkdrop
make dev
# Runs on http://localhost:6767
```

### Use FtR

```bash
# Login
ftr login
# (prompts for email and password)

# Search for packages
ftr search "database"

# Install one
ftr get someuser/someproject@1.0.0

# Upload your work
ftr up myapp.zip myuser/myrepo

# Edit with team (in browser)
# http://localhost:6767/edit/config.yaml/myuser/myrepo
```

Done. FtR handles encryption, version management, build detection, and all the hard parts.

---

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

**Why this is better:**
- Users don't need build tools
- Cross-platform automatic (Linux, macOS, Windows)
- No documentation to maintain
- Integrity guaranteed with SHA256 verification

---

### Workflow 2: Real-Time Team Editing

**Scenario:** Your team needs to update deployment configs together.

```bash
# Step 1: One person creates/uploads
ftr up deployment-config.yaml myteam/ops

# Step 2: Team opens in browser
# http://localhost:6767/edit/deployment-config.yaml/myteam/ops

# Everyone sees:
# ✓ Live updates (changes appear instantly)
# ✓ Who's editing (multiple cursors with names)
# ✓ Syntax highlighting (YAML, JSON, code files)
# ✓ Version history (all changes tracked)
# ✓ Auto-save (every 30 seconds)

# Step 3: Download the final version
ftr down myteam/ops
```

**Why this is better:**
- Faster than email or Git commits
- Easy to see changes as they happen
- Team members don't need Git knowledge
- Built-in audit trail

---

### Workflow 3: Secure Binary Distribution

**Scenario:** You distribute proprietary tools to 100 developers securely.

```bash
# Step 1: Build and encrypt
ftr pack ./secure-tool
ftr up ./secure-tool.sqar company/tools -E   # -E = AES-256 encrypt

# Step 2: Developers download (decryption automatic)
ftr get company/tools

# FtR automatically:
# ✓ Decrypts on download
# ✓ Verifies SHA256 hash
# ✓ Detects tampering
# ✓ Installs to ~/.local/ftr/
# ✓ Tracks version locally

# Step 3: Distribute new version
ftr up ./new-version.sqar company/tools -E

# Developers get updates
ftr list --upgradeable
ftr upgrade company/tools
```

**Why this is better:**
- Files encrypted in transit (server never sees plaintext)
- Developers hold encryption keys (you don't manage them)
- Automatic version tracking
- SHA256 integrity guaranteed
- Works on any network (no VPN required)

---

### Workflow 4: Mount Repos in File Manager

**Scenario:** Your team wants to work with files using Finder/Explorer, not CLI.

```bash
# Step 1: Mount the repository
ftr mount myuser/myrepo ~/mnt/myrepo

# Step 2: Use file manager normally
# - Drag files in → auto-uploaded
# - Edit files locally → synced to server
# - Copy/paste works normally
# - Can use VSCode, Sublime, etc. on mounted files

# Step 3: Auto-mount on system startup
ftr automount install myuser/myrepo ~/mnt/myrepo
ftr automount enable myuser/myrepo
# Now repository is always available when you boot
```

**Why this is better:**
- No learning curve for non-developers
- Works like Dropbox/OneDrive (but for code repos)
- Can use any app (VSCode, Finder, etc.)
- Desktop notifications on changes

---

### Workflow 5: CI/CD Integration

**Scenario:** Your GitHub Actions need to upload build artifacts.

```yaml
# In your .github/workflows/build.yml
- name: Upload build artifacts to FtR
  run: |
    ftr login --token ${{ secrets.FTR_TOKEN }}
    ftr up ./build-output myorg/builds
    ftr up ./docker.tar myorg/containers

# Later in production
- name: Download and deploy
  run: |
    ftr get myorg/builds@${{ github.ref_name }}
    ftr get myorg/containers@${{ github.ref_name }}
    ./deploy.sh
```

**Why this is better:**
- Single credential to manage (not S3, Docker Hub, etc.)
- Works the same everywhere (laptop, CI, production)
- Automatic versioning
- Encrypted transport optional

---

## Feature Guide

### Package Management

**Download & Install**
```bash
ftr get myuser/myrepo              # Latest version
ftr get myuser/myrepo@1.5.0        # Specific version
ftr get myuser/myrepo -A           # Interactive file selection
ftr get myuser/myrepo -D           # Decrypt if encrypted
```

**Upload & Share**
```bash
ftr up myapp.zip myuser/myrepo           # Plain upload
ftr up myapp.zip myuser/myrepo -E        # Encrypted
ftr up file1.tar file2.zip myuser/repo   # Multiple files
```

**Version Management**
```bash
ftr list                           # Installed packages
ftr list --upgradeable             # Available upgrades
ftr upgrade myuser/myrepo          # Upgrade specific
ftr upgrade --all                  # Upgrade everything
ftr remove myuser/myrepo           # Uninstall
```

**Search & Discover**
```bash
ftr search "database"              # Find packages
ftr query myuser/myrepo            # Get package info
```

---

### Collaborative Editing

**Open for Editing**
- Browse to `http://localhost:6767`
- Navigate to your repository
- Click any text file
- Click "Edit"

**Supported File Types**
- Code: Go, Python, JavaScript, TypeScript, C++, Java, Rust
- Markup: HTML, XML, Markdown
- Config: YAML, JSON, TOML, INI, ENV
- Documents: Plain text, CSS, SQL
- Archives: 5MB limit (larger files use document editor)

**Real-Time Features**
- Multiple cursors with user names
- Live updates (no refresh needed)
- Syntax highlighting
- Auto-save every 30 seconds
- Full version history
- Conflict resolution

---

### Repository Management

**Create & Share**
```
1. Login to http://localhost:6767
2. Click "New Repository"
3. Set name, description, visibility
4. Upload files via drag-and-drop
5. Click "Settings" to add co-owners
```

**Permissions**
- **Private**: Only listed owners can access
- **Public**: Discoverable and readable by anyone
- **Multi-owner**: Add colleagues as co-owners

**Download**
- Single files: Click file, then "Download"
- Entire repository: Click folder, then "Download as ZIP"
- Via CLI: `ftr down myuser/myrepo`

---

### Profile & Contact System

**Your Profile**
```bash
# Login to http://localhost:6767
# Click "Settings" → "Manage Profile"
# - Upload profile picture (auto-cropped to 200×200)
# - Add bio (max 500 characters)
# - Profile visible to other users
```

**Contacts**
```bash
# In profile settings
# - Search for other users
# - Add as contacts (trusted collaborators)
# - When sharing repos, filter by contacts
# - Contact list prevents accidental sharing
```

---

### Encryption

**When to Encrypt**
- Proprietary source code
- API keys and credentials
- Personal/sensitive data
- Anything you don't want the server to see plaintext

**How to Encrypt**
```bash
# Upload with encryption
ftr up sensitive.zip myrepo -E

# Server never sees plaintext
# Decryption happens automatically on download
# Encryption keys stored locally in ~/.config/ftr/
```

---

### FUSE Mounting

**Mount Repository**
```bash
ftr mount myuser/myrepo ~/mnt/myrepo
# Now ~/mnt/myrepo/ acts like a normal directory
```

**Use Normally**
- Open files in any editor
- Drag files in (auto-uploaded)
- Edit and save (auto-synced)
- Works with VSCode, Sublime, etc.

**Auto-Mount on Startup**
```bash
ftr automount install myuser/myrepo ~/mnt/myrepo
ftr automount enable myuser/myrepo
# Now auto-mounts when system boots
```

---

## Installation

### FtR CLI

**Linux/macOS:**
```bash
git clone https://github.com/your-org/ftr-project
cd ftr-project/ftr
./install.sh
ftr version  # Verify
```

**Manual build:**
```bash
cd ftr-project/ftr
go build -o ftr ./cmd/main.go
sudo install -m 755 ftr /usr/local/bin/ftr
```

### InkDrop Server

**Development:**
```bash
cd ftr-project/inkdrop
go build -o inkdrop ./cmd/main.go
./inkdrop
# Runs on http://localhost:6767
```

**Production:**
```bash
cd ftr-project/inkdrop
make build
make start   # Via systemd
systemctl status inkdrop
```

### Server Configuration

**Storage location** (default: `/srv/ftr/`)
```bash
export FTR_ROOT_DIR=/custom/path
export FTR_PFP_DIR=/custom/path/pfp   # Profile pictures
```

**Default profile picture**
```bash
# Place in inkdrop/assets/
mkdir -p inkdrop/assets
cp your-image.png inkdrop/assets/default.png
```

---

## Usage Examples

### Example 1: Solo Developer Share

```bash
# Your project
my-app/
├── main.go
├── README.md
└── install.sh

# Upload
ftr pack ./my-app
ftr up ./my-app.sqar john/my-app -E

# User downloads
ftr get john/my-app
# ✓ Auto-detects Go
# ✓ Compiles for their platform
# ✓ Binary ready to use
```

### Example 2: Team Configuration Management

```bash
# Create repo
ftr up config.yaml myteam/ops

# Team edits together
# http://localhost:6767/edit/config.yaml/myteam/ops

# Deploy with new config
ftr down myteam/ops
cd ops
./deploy.sh config.yaml
```

### Example 3: Secure Tool Distribution

```bash
# Build and encrypt
ftr pack ./security-tool
ftr up ./security-tool.sqar acme/tools -E

# 100 developers download
ftr get acme/tools

# Auto-receives updates
ftr upgrade acme/tools
```

### Example 4: Design Team Collaboration

```bash
# Designer uploads design assets
ftr up mockups.figma design/website

# Everyone views in real-time
ftr mount design/website ~/mnt/design

# Team edits documents
http://localhost:6767/edit/README.md/design/website

# Developers download final version
ftr get design/website
```

---

## Technology Stack

| Component | Technology | Version |
|-----------|-----------|---------|
| **FtR CLI** | Go | 1.24.8 |
| **InkDrop Server** | Go | 1.25.6 |
| **Router** | Chi | v5 |
| **Database** | SQLite | Latest |
| **ORM** | bun | Latest |
| **Encryption** | AES-256-CBC | - |
| **Hashing** | bcrypt | - |
| **Upload Protocol** | TUS | - |
| **Real-Time** | Server-Sent Events (SSE) | - |
| **FUSE Mounting** | bazil.org/fuse | - |
| **Editor** | Ace (embedded) | - |

---

## License

**MIT License with Commons Clause** — Use and modify freely, but cannot commercially redistribute the software itself.

For full terms, see LICENSE file in repository.

---

*FtR Project © 2026 Quan Thai. Unified software distribution, collaborative development, and file repository management in one ecosystem.*
