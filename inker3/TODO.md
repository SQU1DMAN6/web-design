# Inker 3 Rebuild Checklist

## Phase 1: Fix Imports & Dependencies
- [x] Install chokidar + uuid (pnpm install)
- [ ] Rewrite sync-manager.js to remove chokidar dependency, use fs.watch + periodic poll
- [ ] Remove chokidar from dependencies (no longer needed)

## Phase 2: Session Confirmation
- [ ] Add sessionConfirm() method to api-client.js
- [ ] Add session confirmation on startup in main.js
- [ ] Update preload.js with session checking

## Phase 3: Simplify UI (Remove File Browser)
- [ ] Rewrite renderer/index.html - clean panels only
- [ ] Rewrite renderer/renderer.js - new UI logic
- [ ] Update renderer/style.css - new styles

## Phase 4: Fix MSI Build
- [ ] Update package.json with electron-builder config
- [ ] Fix FtR_Inker.wxs to use bundled Electron
- [ ] Rewrite build_msi.bat
- [ ] Use icon.png for app icon
- [ ] Add postinstall build step

## Phase 5: Tests & Verification
- [ ] Verify app runs with `npm start`
- [ ] Verify MSI builds
- [ ] Update README/docs