# InkDrop 4.0: Revamp Update Specification

## Overview

InkDrop 4.0 is a complete platform revamp focused on transforming InkDrop from a repository browser into a modern collaborative workspace.

The goal is to evolve the existing Drop-based storage system into a structured environment for:

- File management
- Collaborative projects
- Document creation
- Presentations
- Sharing
- Version tracking
- User collaboration
- Project organisation
- Future integrations

InkDrop 4.0 should preserve existing Drop functionality while introducing a new interface, architecture, and workflow model.

The core principle:

> A Drop is no longer simply a folder. A Drop is a workspace.

---

# Proposed Development Order:
Phase 0: Project Preparation
        |
        v
Phase 1: Backend Foundation
        |
        v
Phase 2: Drop System
        |
        v
Phase 3: Permission System
        |
        v
Phase 4: File System Migration
        |
        v
Phase 5: API v4
        |
        v
Phase 6: Frontend Shell
        |
        v
Phase 7: Workspace Features
        |
        v
Phase 8: Collaboration

---

# 1. Core Terminology Changes

## Repository → Drop

The internal repository system remains compatible, but all user-facing terminology changes.

Old:

    Repository
    Repo
    Repository Browser

New:

    Drop
    Workspace
    Drop Browser

A Drop represents a complete workspace owned by one or more users.

---

# 2. New Application Layout

InkDrop 4.0 introduces a four-panel interface.

The layout is persistent across the application.

+------------------------------------------------+
| Navigation Panel |
+------------------------------------------------+
| Drop Information Panel |
+----------------------+-------------------------+
| Project Manager | Workspace View |
+----------------------+-------------------------+

---

# 3. Navigation Panel

The Navigation Panel is always visible.

Purpose:

- Global application navigation
- User account management
- Page navigation
- System actions

Features:

## User Profile

Clicking the profile opens:

- Account settings
- Profile customisation
- Security settings
- Connected applications
- Logout

## Browser Controls

Include:

- Back
- Forward
- Refresh
- Home

These should behave similarly to browser navigation.

## Global Navigation

Options:

- Home
- My Drops
- Shared With Me
- Public Drops
- Recent Activity
- Settings

---

# 4. Drop Information Panel

The Drop Panel displays current Drop information.

Always visible while inside a Drop.

Information displayed:

- Drop name
- Owner(s)
- Description
- Visibility status
- Creation date
- Last updated time
- Storage usage
- User permissions

Actions:

- Drop settings
- Share Drop
- Copy Drop link
- Download Drop
- View activity

---

# 5. Project Manager Panel

The Project Manager replaces the current file browser controls.

Purpose:

Manage project content types.

Options:

## Create

Buttons:

- New Document
- New Presentation
- New Folder
- Upload Files
- Import Project

## Project Structure

Displays:

- Files
- Documents
- Presentations
- Assets
- Trash

Tree navigation should support:

- Expanding folders
- Dragging files
- Moving items
- Renaming items

---

# 6. Workspace View

The Workspace View is the main content area.

Different content types open here.

Supported views:

## File Browser

Modern replacement for current directory browser.

Features:

- Grid view
- List view
- Search
- Sorting
- Filtering
- File previews

## Document Editor

Native InkDrop document editor.

Features:

- Rich text editing
- Markdown support
- Autosave
- Collaboration
- Comments
- Version history

## Presentation Editor

Create slide-based content.

Features:

- Slide management
- Themes
- Images
- Text boxes
- Export support

---

# 7. Drop Metadata System

The existing metadata system is expanded.

Current:

meta.json

New:

drop.json

Example:

{
    "name": "Project Alpha",
    "owners": [
        "user1"
    ],
    "description": "",
    "visibility": "private",
    "created": 0,
    "updated": 0,
    "settings": {
        "allow_comments": true,
        "allow_uploads": false
    }
}

---

# 8. Permission Model

Replace simple ownership checks with role-based permissions.

Roles:

## Owner

Can:

- Delete Drop
- Manage permissions
- Change settings
- Transfer ownership

## Editor

Can:

- Create files
- Modify files
- Delete files
- Upload content

## Viewer

Can:

- View files
- Download allowed content

---

# 9. Sharing System

Drop sharing becomes a first-class feature.

Support:

- User sharing
- Contact sharing
- Public links
- Permission selection

Example:

Share Drop:

    User: alice
    Permission: Editor

---

# 10. API Revamp

Existing API routes remain supported.

New API structure:

    /api/v4/

Examples:

    GET /api/v4/drops

Returns available Drops.

    GET /api/v4/drop/{id}

Returns Drop metadata.

    GET /api/v4/drop/{id}/files

Returns files.

    POST /api/v4/drop/{id}/share

Shares Drop.

---

# 11. Existing Compatibility

InkDrop 4.0 must maintain compatibility with:

Existing routes:

    /login
    /register
    /browse/{user}/{drop}
    /api/fs
    /preview
    /download

Old clients must continue functioning.

Compatibility routes should internally redirect to the new API layer.

---

# 12. File System Improvements

Current filesystem operations are retained:

- Create directory
- Create file
- Rename
- Delete
- Restore
- Trash

New requirements:

## File IDs

Every object receives a unique identifier.

Example:

file:

    {
        "id": "abc123",
        "name": "main.go",
        "type": "code"
    }

This prevents problems when files are renamed.

---

# 13. Version History

Every editable object supports versions.

Requirements:

- Automatic snapshots
- Manual checkpoints
- Restore previous versions
- View changes

Example:

Version 12:

    Updated login system

Version 11:

    Added account settings

---

# 14. Activity System

Every Drop maintains activity history.

Events:

- File created
- File modified
- File deleted
- User joined
- Permission changed

Example:

    qchef edited presentation.pptx

---

# 15. Search System

Global search across:

- Files
- Drops
- Users
- Documents

Search should support:

- Filename search
- Content search
- Owner filtering
- Date filtering

---

# 16. Security Requirements

InkDrop 4.0 must improve security.

Required:

## Authentication

- Secure session handling
- Session expiration
- Account recovery

## Authorization

Every action must verify:

- User identity
- Drop permission
- Object permission

## File Security

Prevent:

- Path traversal
- Unauthorized downloads
- Unsafe uploads

## Upload Protection

Requirements:

- File size limits
- Extension validation
- Malware scanning support
- Safe filenames

---

# 17. Backend Architecture

Recommended structure:

    inkdrop/

    controller/
        account/
        auth/
        drop/
        files/
        collaboration/

    service/

        drop_service
        permission_service
        file_service
        version_service

    model/

        user
        drop
        file
        permission

    repository/

        filesystem
        database

---

# 18. Database Expansion

Move metadata from only JSON storage into database-backed records.

Required tables:

Users

Drops

DropMembers

Files

FileVersions

ActivityLogs

Shares

Sessions

---

# 19. Frontend Requirements

The new interface should be:

- Responsive
- Fast
- Keyboard friendly
- Accessible

Required:

- Dark mode
- Drag and drop
- Context menus
- Keyboard shortcuts
- Live updates

---

# 20. Migration Plan

Migration from InkDrop 3.x:

Step 1:

Detect existing repository folders.

Step 2:

Generate Drop metadata.

Step 3:

Create file indexes.

Step 4:

Assign ownership.

Step 5:

Enable new interface.

No user data should be lost.

---

# 21. Development Milestones

## Phase 1: Foundation

Complete:

- New routing system
- Drop model
- Permission service
- Updated metadata

## Phase 2: New Interface

Complete:

- Four-panel layout
- Drop dashboard
- Project manager

## Phase 3: Collaboration

Complete:

- Sharing
- Roles
- Activity

## Phase 4: Advanced Features

Complete:

- Documents
- Presentations
- Version history
- Search

---

# 22. Existing Codebase Migration

InkDrop 4.0 must migrate the current InkDrop 3.x architecture.

Current:

controller/
    repository/

repository/
    filesystem operations

model/
    user handling


New:

controller/
    drop/
    file/
    collaboration/

service/
    drop/
    permission/
    activity/
    version/

storage/
    filesystem/
    database/


Controllers should never directly manipulate filesystem storage.

All operations must pass through services.

Example:

Old:

HTTP Request
    |
Repository Controller
    |
Filesystem


New:

HTTP Request
    |
Controller
    |
Service Layer
    |
Storage Layer

# 23. Drop Architecture

A Drop is the primary workspace object.

A Drop contains:

Drop
|
+-- Metadata
|
+-- Members
|
+-- Files
|
+-- Documents
|
+-- Presentations
|
+-- Activity Log
|
+-- Versions
|
+-- Settings


Every Drop must have:

- Unique ID
- Owner
- Creation timestamp
- Modification timestamp
- Permission list
- Storage location

# 24. Frontend Component Architecture

The four-panel system should be implemented as independent components.

## NavigationPanel

Responsible for:

- User menu
- Global navigation
- History controls


## DropPanel

Responsible for:

- Drop information
- Sharing
- Settings


## ProjectManager

Responsible for:

- File tree
- Creation actions
- Drag and drop


## Workspace

Responsible for:

- Editors
- Preview
- File browsing
- Applications


Communication:

NavigationPanel
        |
        v
Application State Manager
        |
        +---- DropPanel
        |
        +---- ProjectManager
        |
        +---- Workspace
        
# 25. Real-Time System

InkDrop 4.0 should support real-time events.

Transport:

WebSocket


Events:

drop.updated

file.created

file.modified

file.deleted

member.joined

permission.changed


Example:

User A edits a document.

Server:

document.updated

      |

WebSocket

      |

User B receives update.

# 26. Content Object Architecture

InkDrop 4.0 introduces multiple content types.

All content inside a Drop must inherit from a common object model.

Base Object:

{
    "id": "unique-id",
    "drop_id": "drop-id",
    "name": "object-name",
    "type": "file|document|presentation|folder",
    "created": 0,
    "updated": 0,
    "created_by": "user-id"
}

---

# Document Architecture

Documents are first-class InkDrop objects.

Requirements:

- Autosave
- Collaborative editing
- Conflict resolution
- Version snapshots
- Comment anchoring
- Export support

---

# Presentation Architecture

Presentations are slide-based objects.

Requirements:

- Slide ordering
- Templates
- Themes
- Media embedding
- Export to PDF
- Version history


---

# 27. Storage Architecture

InkDrop uses a hybrid storage system.

Database:

Stores:

- Users
- Permissions
- Metadata
- Versions
- Activity
- Relationships


Filesystem:

Stores:

- Uploaded files
- Large binary objects
- Document assets
- Presentation media


Structure:

storage/

drops/

    {drop_id}/

        files/

        assets/

        documents/

        presentations/


Database references filesystem objects.

Controllers must never access storage directly.

---

# 28. API Standards

All v4 APIs follow:

/api/v4/


Response format:

Success:

{
    "success": true,
    "data": {}
}


Error:

{
    "success": false,
    "error": {
        "code": "PERMISSION_DENIED",
        "message": "User cannot edit this Drop"
    }
}


Required:

- HTTP status codes
- Request validation
- Authentication middleware
- Permission middleware
- Rate limiting


---

# 29. Application State Management

The frontend requires a central state manager.

State:

ApplicationState

Contains:

UserState

- Current user
- Authentication status


DropState

- Current Drop
- Members
- Permissions


WorkspaceState

- Current file
- Current editor
- Current view


UIState

- Active panel
- Theme
- Dialogs
- Notifications


Flow:

User Action

|

Component

|

State Manager

|

API Service

|

Backend

|

State Update


---

# 31. Event System Architecture

All system changes create events.

Event format:

{
    "event": "file.created",
    "drop_id": "abc123",
    "user": "user1",
    "timestamp": 0,
    "data": {}
}


Events are used for:

- Activity logs
- WebSocket updates
- Notifications
- Auditing


Event pipeline:

Action

|

Service Layer

|

Event Dispatcher

|

+-- Database Activity Log

+-- WebSocket Broadcast

+-- Notification System


---

# 32. Logging Requirements

InkDrop must maintain structured logs.

Required logs:

Authentication:

- Login attempts
- Session creation
- Password recovery


Security:

- Permission failures
- Suspicious uploads
- Invalid requests


System:

- API errors
- Database failures
- Storage failures


Logs must not contain:

- Passwords
- Authentication tokens
- Private file contents


---

# 33. Deployment Architecture

Recommended production structure:

Client

|

Reverse Proxy

|

API Server

|

+-- Database

+-- File Storage

+-- WebSocket Server


Components:

Frontend:

- Static web application


Backend:

- REST API
- Authentication
- Services


Database:

- PostgreSQL recommended


Storage:

- Local filesystem or object storage


---

# 34. Backup and Recovery

InkDrop must support:

Database backups:

- User data
- Metadata
- Permissions


Storage backups:

- Files
- Documents
- Presentations


Recovery requirements:

- Restore complete Drop
- Restore individual files
- Restore previous versions


---

# 35. Final Development Principle

InkDrop 4.0 development must follow:

Controllers handle requests.

Services contain logic.

Repositories handle storage.

Models represent data.

Frontend components display state.

No layer should bypass another layer.

InkDrop 4.0 should be designed as a scalable collaboration platform, not a collection of file operations.

# Success Criteria

InkDrop 4.0 is successful when:

- Existing users can migrate without data loss
- Drops feel like complete workspaces
- Collaboration is simple
- File management is intuitive
- Permissions are secure
- The interface competes with modern cloud workspace platforms

InkDrop 4.0 is not a file browser.

It is a collaborative project environment built around Drops.