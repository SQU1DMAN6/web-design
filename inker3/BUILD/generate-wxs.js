const fs = require('fs');
const path = require('path');

function findAppDir() {
    // Possible directory names (with/without leading space from quoting issues)
    const distDir = path.resolve(__dirname, '..', 'dist');
    if (!fs.existsSync(distDir)) return null;
    const entries = fs.readdirSync(distDir);
    for (const entry of entries) {
        if (entry.endsWith('-win32-x64') && entry.includes('Inker')) {
            // Check for exe with matching name
            const exeCandidates = [
                'FtR Inker.exe',
                ' FtR Inker.exe',
                entry.replace(/-win32-x64$/, '.exe'),
            ];
            const fullPath = path.join(distDir, entry);
            for (const exe of exeCandidates) {
                if (fs.existsSync(path.join(fullPath, exe))) return fullPath;
            }
        }
    }
    return null;
}

const APP_DIR = findAppDir();
if (!APP_DIR) { console.error('ERROR: Run "make build" first.'); process.exit(1); }
console.log('App dir: ' + APP_DIR);

// Collect files from the packaged app directory.
// Handles both resources/app.asar (single file) and resources/app/ (directory).
const allFiles = [];

// Directories and files to skip inside resources/app/ (dev-only, not needed at runtime)
const SKIP_DIRS = ['node_modules', 'BUILD', '.git', '.pnpm-store'];
const SKIP_FILES = ['electron-builder.yml', 'Makefile', 'TODO.md', 'pnpm-lock.yaml', 'pnpm-workspace.yaml'];

function scan(dir, subdir, depth) {
    for (const e of fs.readdirSync(dir)) {
        if (e === '.' || e === '..') continue;
        const fp = path.join(dir, e);
        const stat = fs.statSync(fp);
        if (stat.isDirectory()) {
            if (depth === 0 && (e === 'resources' || e === 'locales')) {
                scan(fp, e, 1);
            } else if (depth === 1 && subdir === 'resources' && e === 'app') {
                // resources/app is a directory (not app.asar) — fully recurse into it
                // to include main.js, preload.js, lib/, renderer/, etc.
                scan(fp, path.join(subdir, e), 2);
            } else if (depth >= 2 && subdir && subdir.startsWith('resources\\app')) {
                // Skip dev-only directories inside resources/app
                if (SKIP_DIRS.indexOf(e) !== -1) continue;
                scan(fp, path.join(subdir, e), depth + 1);
            }
        } else {
            // Skip dev-only files inside resources/app
            if (subdir && subdir.startsWith('resources\\app') && depth === 2 && SKIP_FILES.indexOf(e) !== -1) {
                continue;
            }
            const d = subdir || '.';
            if (!allFiles[d]) allFiles[d] = [];
            allFiles[d].push({ name: e, fullpath: fp.replace(/\\/g, '\\\\') });
        }
    }
}
scan(APP_DIR, null, 0);

const totalFiles = Object.values(allFiles).reduce((a, b) => a + b.length, 0);
console.log('Files: ' + totalFiles);

const dirs = Object.keys(allFiles).sort(function(a, b) {
    return a.split('\\').length - b.split('\\').length || a.localeCompare(b);
});

let compIdx = 0;
let wxs = `<?xml version="1.0" encoding="UTF-8"?>
<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <Package Name="FtR Inker" Language="1033" Version="3.2.0" Manufacturer="Quan Thai" UpgradeCode="5da499e7-668a-4df9-8a69-f1b6310fd334">
    <MajorUpgrade DowngradeErrorMessage="A newer version is already installed." />
    <MediaTemplate EmbedCab="yes" />
    <Feature Id="ProductFeature" Title="FtR Inker" Level="1">\n`;

for (const dir of dirs) {
    for (const f of allFiles[dir]) {
        wxs += `      <ComponentRef Id="C${compIdx}" />\n`;
        compIdx++;
    }
}
wxs += `      <ComponentRef Id="Start" />
      <ComponentRef Id="Desktop" />
      <ComponentRef Id="AutoRun" />
    </Feature>
    <StandardDirectory Id="ProgramFiles64Folder">
      <Directory Id="INSTALLDIR" Name="FtR\\Inker">\n`;

// Build directory tree from dirs array
compIdx = 0;
const tree = {};
tree['.'] = { _files: allFiles['.'] || [] };

for (const dir of dirs) {
    if (dir === '.') continue;
    const parts = dir.split('\\');
    let node = tree;
    for (let i = 0; i < parts.length; i++) {
        if (!node[parts[i]]) {
            node[parts[i]] = { _files: [] };
        }
        if (i === parts.length - 1) {
            node[parts[i]]._files = allFiles[dir] || [];
        }
        node = node[parts[i]];
    }
}

// Recursively emit directories and components with proper nesting
function emitDir(dirNode, dirName, dirId, depth) {
    // Emit components for files in this directory
    for (const f of dirNode._files || []) {
        wxs += '        '.repeat(depth) + `<Component Id="C${compIdx}" Guid="*">\n`;
        wxs += '        '.repeat(depth + 1) + `<File Id="F${compIdx}" Name="${f.name}" Source="${f.fullpath}" KeyPath="yes" />\n`;
        wxs += '        '.repeat(depth) + `</Component>\n`;
        compIdx++;
    }

    // Emit child directories
    const childDirs = Object.keys(dirNode).filter(function(k) { return k !== '_files'; }).sort();
    for (const childName of childDirs) {
        const childId = dirId ? dirId + '_' + childName.replace(/[^a-zA-Z0-9]/g, '_') : 'D' + childName.replace(/[^a-zA-Z0-9]/g, '_');
        wxs += '        '.repeat(depth) + `<Directory Id="${childId}" Name="${childName}">\n`;
        emitDir(tree_getNode(dirNode, childName), childName, childId, depth + 1);
        wxs += '        '.repeat(depth) + `</Directory>\n`;
    }
}

function tree_getNode(treeNode, name) {
    return treeNode[name];
}

// Emit root-level components first
emitDir(tree['.'], '.', null, 3);

// Emit subdirectories
const topLevelDirs = Object.keys(tree).filter(function(k) { return k !== '.'; }).sort();
for (const topDir of topLevelDirs) {
    const topId = 'D' + topDir.replace(/[^a-zA-Z0-9]/g, '_');
    wxs += '        '.repeat(3) + `<Directory Id="${topId}" Name="${topDir}">\n`;
    emitDir(tree[topDir], topDir, topId, 4);
    wxs += '        '.repeat(3) + `</Directory>\n`;
}

wxs += `      </Directory>
    </StandardDirectory>
    <StandardDirectory Id="ProgramMenuFolder">
      <Directory Id="AppMenu" Name="FtR Inker">
        <Component Id="Start" Guid="7a6bb7af-9f82-4e4f-8352-d6fa850a9fc3">
          <Shortcut Id="MenuShortcut" Name="FtR Inker" Description="FtR Inker Client" Target="[INSTALLDIR]FtR Inker.exe" WorkingDirectory="INSTALLDIR" />
          <RemoveFolder Id="CleanUp" Directory="AppMenu" On="uninstall" />
          <RegistryValue Root="HKCU" Key="Software\\FtR\\Inker" Name="Installed" Type="integer" Value="1" />
        </Component>
      </Directory>
    </StandardDirectory>
    <StandardDirectory Id="DesktopFolder">
      <Component Id="Desktop" Guid="60992b99-8521-4ba2-bf3d-f1de9d27f61c">
        <Shortcut Id="DesktopShortcut" Name="FtR Inker" Description="FtR Inker Client" Target="[INSTALLDIR]FtR Inker.exe" WorkingDirectory="INSTALLDIR" />
        <RegistryValue Root="HKCU" Key="Software\\FtR\\Inker" Name="DesktopShortcut" Type="integer" Value="1" />
      </Component>
    </StandardDirectory>
    <Component Id="AutoRun" Guid="b3f17e2a-8d4c-4c5e-9f1a-2e7b8c9d0e1f">
      <RegistryValue Root="HKCU" Key="Software\\Microsoft\\Windows\\CurrentVersion\\Run" Name="FtR Inker" Value="[INSTALLDIR]FtR Inker.exe" Type="string" />
    </Component>
  </Package>
</Wix>`;

const out = path.join(__dirname, 'FtR_Inker_full.wxs');
fs.writeFileSync(out, wxs);
console.log('Generated: ' + out);
console.log('Components: ' + compIdx);