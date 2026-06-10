# The FtR Project

FtR and InkDrop work together as a practical file-sharing and collaboration toolset for users who need to move files, package releases, mount Drops, and edit content together.

**Current release:** FtR & InkDrop 3.2

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

Then start working with Drops:

```bash
ftr search database
ftr get someuser/someproject
ftr up myfile.zip myuser/mydrop
```

If you use a shared InkDrop server, log in to that server before browsing or uploading.

## What You Can Do

### Find and install packages

Use `ftr search` to discover Drops, then `ftr get` to download them.

```bash
ftr search golang
ftr get someuser/myproject
ftr get someuser/myproject@1.5.0
```

### Share files and releases

Upload one file or several files to a Drop.

```bash
ftr up myfile.zip myuser/mydrop
ftr up file1.tar file2.zip myuser/mydrop
ftr up myfile.zip myuser/mydrop -E
```

### Work with remote files

Use `ftr remote` for direct file and folder actions on a Drop.

```bash
ftr remote down user/drop/path/to/file.txt
ftr remote mkdir user/drop/new-folder
ftr remote rename user/drop/old-name.txt new-name.txt
ftr remote delete user/drop/file-to-remove.txt
```

### Mount a Drop locally

Mount a Drop if you want to work with it through your file manager or editor.

```bash
ftr mount myuser/mydrop ~/mnt/mydrop
```

### Keep installed packages current

```bash
ftr list
ftr list --upgradeable
ftr upgrade myuser/mydrop
ftr upgrade --all
```

## Using InkDrop Well

InkDrop is the place where you browse Drops, open files, upload content, and collaborate in the browser.

Use it when you need to:

- review files before downloading them
- edit shared text files with others
- upload content into a Drop
- browse Drop contents from the web
- manage files without staying in the terminal

Practical habits that help:

- keep Drops named clearly
- upload release files with version numbers in the filename
- use `ftr list --upgradeable` when you want to check what needs updating
- use `ftr remote down` for single-file retrieval instead of downloading an entire Drop

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

### Mount a team Drop

```bash
ftr mount design/team-assets ~/mnt/team-assets
```

### Download one file from a Drop

```bash
ftr remote down user/drop/config.yaml
```

## Common Commands

```bash
ftr login                  # Sign in
ftr search <query>         # Search Drops
ftr get <user/drop>        # Download a Drop package
ftr up <file> <user/drop>  # Upload a file
ftr list                   # Show installed packages
ftr list --upgradeable     # Show available updates
ftr upgrade <package>      # Upgrade an installed package
ftr mount <drop> <path>    # Mount a remote Drop
ftr remote down <path>     # Download a single remote file
ftr remote mkdir <path>    # Create a remote folder
ftr remote rename ...      # Rename a remote file or folder
ftr remote delete ...      # Delete remote files
```

## Command Reference

### `ftr search`

Search Drops by name or description.

### `ftr get`

Download a Drop package, or a specific version with `@version`.

### `ftr up`

Upload one or more files to a Drop. Add `-E` to upload an encrypted archive.

### `ftr list`

Show installed packages, or only upgradeable ones with `--upgradeable`.

### `ftr upgrade`

Update installed packages to newer versions from their source Drops.

### `ftr mount`

Mount a remote Drop at a local path.

### `ftr remote`

Manage remote files and directories directly from the command line.