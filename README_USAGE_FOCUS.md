# FtR Project - Usage Guide

This guide focuses on how to use FtR and InkDrop as a working system for sharing files, managing packages, and collaborating on repository content.

**Current release:** FtR & InkDrop 3.1

## Contents

1. [Get Started](#get-started)
2. [Everyday Tasks](#everyday-tasks)
3. [InkDrop in Practice](#inkdrop-in-practice)
4. [Command Reference](#command-reference)
5. [Examples](#examples)

## Get Started

Install the CLI:

```bash
curl https://quanthai.net/installftr.sh | sh
```

Sign in:

```bash
ftr login
```

Begin with search or a direct download:

```bash
ftr search database
ftr get someuser/someproject
```

## Everyday Tasks

### Find repositories

Use `ftr search` to locate repositories by name or description.

```bash
ftr search golang
ftr search design assets
```

### Download packages

Use `ftr get` for the latest version or a specific tagged version.

```bash
ftr get user/repo
ftr get user/repo@1.5.0
```

### Upload files

Use `ftr up` to send one or more files into a repository.

```bash
ftr up notes.txt team/docs
ftr up release.tar company/tools -E
ftr up file1.zip file2.zip team/archive
```

### Check what is installed

```bash
ftr list
ftr list --upgradeable
ftr upgrade user/repo
ftr upgrade --all
```

### Work with individual remote files

```bash
ftr remote down user/repo/path/to/file.txt
ftr remote mkdir user/repo/new-folder
ftr remote rename user/repo/old-name.txt new-name.txt
ftr remote delete user/repo/old-name.txt
```

### Mount a repository locally

```bash
ftr mount user/repo ~/mnt/repo
```

## InkDrop in Practice

InkDrop is useful when you want to work in a browser instead of the terminal.

Typical use:

1. Open InkDrop in your browser.
2. Go to the repository you want to work with.
3. Open a file to preview or edit it.
4. Share the same file with a teammate for live collaboration.
5. Upload or download files as needed.

Use it for:

- browsing repositories
- editing shared text files
- reviewing content before download
- organizing repository files
- moving between browser and local tools without changing the workflow

## Command Reference

```bash
ftr login                      # Sign in
ftr search <query>             # Search repositories
ftr get <user/repo>[@version]  # Download a repository package
ftr up <file...> <user/repo>   # Upload files
ftr list                       # Show installed packages
ftr list --upgradeable         # Show updates
ftr upgrade <package...>       # Upgrade packages
ftr mount <repo> [mountpoint]   # Mount a repository locally
ftr remote down <path>          # Download one remote file
ftr remote mkdir <path>         # Create a remote folder
ftr remote rename ...           # Rename a remote path
ftr remote delete ...           # Remove remote files
```

## Examples

### Share a release

```bash
ftr up build/app.tar team/releases
```

### Download a specific version

```bash
ftr get team/tooling@2.3.1
```

### Mount shared assets

```bash
ftr mount design/assets ~/mnt/design-assets
```

### Remove a remote file

```bash
ftr remote delete team/releases/old-build.tar
```

