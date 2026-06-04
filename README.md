# The FtR Project

FtR and InkDrop work together as a practical file-sharing and collaboration toolset for users who need to move files, package releases, mount remote repositories, and edit content together.

**Current release:** FtR & InkDrop 3.1

## Contents

1. [Quick Start](#quick-start)
2. [What You Can Do](#what-you-can-do)
3. [Common Commands](#common-commands)
4. [Using InkDrop Well](#using-inkdrop-well)
5. [Examples](#examples)
6. [Command Reference](#command-reference)

## Quick Start

Install the FtR CLI:

```bash
curl https://quanthai.net/installftr.sh | sh
```

Sign in to your InkDrop account:

```bash
ftr login
```

Then start working with repositories:

```bash
ftr search database
ftr get someuser/someproject
ftr up myfile.zip myuser/myrepo
```

If you use a shared InkDrop server, log in to that server before browsing or uploading.

## What You Can Do

### Find and install packages

Use `ftr search` to discover repositories, then `ftr get` to download them.

```bash
ftr search golang
ftr get someuser/myproject
ftr get someuser/myproject@1.5.0
```

### Share files and releases

Upload one file or several files to a repository.

```bash
ftr up myfile.zip myuser/myrepo
ftr up file1.tar file2.zip myuser/myrepo
ftr up myfile.zip myuser/myrepo -E
```

### Work with remote files

Use `ftr remote` for direct file and folder actions on a remote repository.

```bash
ftr remote down user/repo/path/to/file.txt
ftr remote mkdir user/repo/new-folder
ftr remote rename user/repo/old-name.txt new-name.txt
ftr remote delete user/repo/file-to-remove.txt
```

### Mount a repository locally

Mount a repository if you want to work with it through your file manager or editor.

```bash
ftr mount myuser/myrepo ~/mnt/myrepo
```

### Keep installed packages current

```bash
ftr list
ftr list --upgradeable
ftr upgrade myuser/myrepo
ftr upgrade --all
```

## Using InkDrop Well

InkDrop is the place where you browse repositories, open files, upload content, and collaborate in the browser.

Use it when you need to:

- review files before downloading them
- edit shared text files with others
- upload content into a repository
- browse repository contents from the web
- manage files without staying in the terminal

Practical habits that help:

- keep repositories named clearly
- upload release files with version numbers in the filename
- use `ftr list --upgradeable` when you want to check what needs updating
- use `ftr remote down` for single-file retrieval instead of downloading an entire repository

## Examples

### Install and use a package

```bash
curl https://quanthai.net/installftr.sh | sh
ftr login
ftr get someuser/someproject
```

### Share a release

```bash
ftr up build/app.tar myteam/releases
```

### Share an encrypted archive

```bash
ftr up release.sqar company/tools -E
```

### Mount a team repository

```bash
ftr mount design/team-assets ~/mnt/team-assets
```

### Download one file from a repository

```bash
ftr remote down user/repo/config.yaml
```

## Common Commands

```bash
ftr login                  # Sign in
ftr search <query>         # Search repositories
ftr get <user/repo>        # Download a repository package
ftr up <file> <user/repo>   # Upload a file
ftr list                   # Show installed packages
ftr list --upgradeable     # Show available updates
ftr upgrade <package>      # Upgrade an installed package
ftr mount <repo> <path>    # Mount a remote repository
ftr remote down <path>     # Download a single remote file
ftr remote mkdir <path>    # Create a remote folder
ftr remote rename ...      # Rename a remote file or folder
ftr remote delete ...      # Delete remote files
```

## Command Reference

### `ftr search`

Search repositories by name or description.

### `ftr get`

Download a repository package, or a specific version with `@version`.

### `ftr up`

Upload one or more files to a repository. Add `-E` to upload an encrypted archive.

### `ftr list`

Show installed packages, or only upgradeable ones with `--upgradeable`.

### `ftr upgrade`

Update installed packages to newer versions from their source repositories.

### `ftr mount`

Mount a remote repository at a local path.

### `ftr remote`

Manage remote files and directories directly from the command line.

