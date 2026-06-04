# FtR Project

FtR is a user-facing toolkit for finding, downloading, sharing, mounting, and managing repositories in InkDrop.

**Current release:** FtR & InkDrop 3.1

## Start Here

```bash
curl https://quanthai.net/installftr.sh | sh
ftr login
ftr search /
ftr get someuser/someproject
ftr up myfile.zip myuser/myrepo
```

## Core Tasks

### Find something to use

```bash
ftr search golang
ftr search assets
```

You may use the `/` keyword to list all available repositories.

### Download a package

```bash
ftr get user/repo
ftr get user/repo@1.2.0
```

### Upload files

```bash
ftr up report.pdf team/docs
ftr up file1.zip file2.zip team/archive
```

### Keep installs up to date

```bash
ftr list
ftr list --upgradeable
ftr upgrade user/repo
ftr upgrade
```

### Work with remote files

```bash
ftr remote down user/repo/path/to/file.txt
ftr remote mkdir user/repo/new-folder
ftr remote rename user/repo/old-name.txt new-name.txt
ftr remote delete user/repo/old-name.txt
```

### Mount a repository

```bash
ftr mount user/repo ~/mnt/repo
```

## InkDrop Workflow

Use InkDrop when you want to:

- browse repository contents
- preview and edit files in the browser
- collaborate on shared text files
- upload or download repository content
- work with a repository from a local mount point

## Quick Reference

```bash
ftr login
ftr search <query>
ftr get <user/repo>[@version]
ftr up <file...> <user/repo>
ftr list [--upgradeable]
ftr upgrade <package...>
ftr mount <user/repo> [mountpoint]
ftr remote <down|mkdir|rename|delete> ...
```

