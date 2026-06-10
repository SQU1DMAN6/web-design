# FtR Project

FtR is a user-facing toolkit for finding, downloading, sharing, mounting, and managing Drops in InkDrop.

**Current release:** FtR & InkDrop 3.2

## Start Here

```bash
curl https://quanthai.net/installftr.sh | sh
ftr login
ftr search /
ftr get someuser/someproject
ftr up myfile.zip myuser/mydrop
```

## Core Tasks

### Find something to use

```bash
ftr search golang
ftr search assets
```

You may use the `/` keyword to list all available Drops.

### Download a package

```bash
ftr get user/drop
ftr get user/drop@1.2.0
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
ftr upgrade user/drop
ftr upgrade
```

### Work with remote files

```bash
ftr remote down user/drop/path/to/file.txt
ftr remote mkdir user/drop/new-folder
ftr remote rename user/drop/old-name.txt new-name.txt
ftr remote delete user/drop/old-name.txt
```

### Mount a Drop

```bash
ftr mount user/drop ~/mnt/drop
```

## InkDrop Workflow

Use InkDrop when you want to:

- browse Drop contents
- preview and edit files in the browser
- collaborate on shared text files
- upload or download Drop content
- work with a Drop from a local mount point

## Quick Reference

```bash
ftr login
ftr search <query>
ftr get <user/drop>[@version]
ftr up <file...> <user/drop>
ftr list [--upgradeable]
ftr upgrade <package...>
ftr mount <user/drop> [mountpoint]
ftr remote <down|mkdir|rename|delete> ...