const fs = require('fs');
const path = require('path');

function findAppDir() {
    const cands = [
        path.resolve(__dirname, '..', 'dist', 'win-unpacked'),
        path.resolve(__dirname, '..', 'dist', 'FtR Inker-win32-x64'),
    ];
    for (const c of cands) {
        if (fs.existsSync(path.join(c, 'FtR Inker.exe'))) return c;
    }
    return null;
}

const APP_DIR = findAppDir();
if (!APP_DIR) { console.error('ERROR: Run "make build" first.'); process.exit(1); }
console.log('App dir: ' + APP_DIR);

function esc(p) { return p.replace(/\\/g, '\\\\'); }

function rel(p) { return path.relative(APP_DIR, p); }

// Collect only top-level files + direct children of known subdirs (locales, resources)
// Do NOT recursively scan into resources (it contains app.asar + extracted node_modules)
const components = [];

function add(name, fullpath, subdir) {
    components.push({ name, source: esc(fullpath), subdir: subdir || null });
}

// Top-level files
for (const e of fs.readdirSync(APP_DIR)) {
    const fp = path.join(APP_DIR, e);
    if (e === '.' || e === '..') continue;
    if (fs.statSync(fp).isFile()) {
        add(e, fp, null);
    }
}

// Locales - first-level only
const localesDir = path.join(APP_DIR, 'locales');
if (fs.existsSync(localesDir)) {
    for (const e of fs.readdirSync(localesDir)) {
        const fp = path.join(localesDir, e);
        if (e !== '.' && e !== '..' && fs.statSync(fp).isFile()) {
            add(e, fp, 'locales');
        }
    }
}

// Resources - first-level files only (just app.asar)
const resourcesDir = path.join(APP_DIR, 'resources');
if (fs.existsSync(resourcesDir)) {
    for (const e of fs.readdirSync(resourcesDir)) {
        const fp = path.join(resourcesDir, e);
        if (e !== '.' && e !== '..' && fs.statSync(fp).isFile()) {
            add(e, fp, 'resources');
        }
    }
}

console.log('Files: ' + components.length);

let wxs = `<?xml version="1.0" encoding="UTF-8"?>
<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <Package Name="FtR Inker" Language="1033" Version="3.2.0" Manufacturer="FtR" UpgradeCode="5da499e7-668a-4df9-8a69-f1b6310fd334">
    <MajorUpgrade DowngradeErrorMessage="A newer version is already installed." />
    <MediaTemplate EmbedCab="yes" />
    <Feature Id="ProductFeature" Title="FtR Inker" Level="1">\n`;

for (let i = 0; i < components.length; i++) {
    wxs += `      <ComponentRef Id="C${i}" />\n`;
}
wxs += `      <ComponentRef Id="Start" />
      <ComponentRef Id="Desktop" />
      <ComponentRef Id="AutoRun" />
    </Feature>
    <StandardDirectory Id="ProgramFiles64Folder">
      <Directory Id="INSTALLDIR" Name="FtR\\Inker">\n`;

const rootComps = components.filter(c => !c.subdir);
const subdirComps = {};
for (const c of components) {
    if (c.subdir) {
        if (!subdirComps[c.subdir]) subdirComps[c.subdir] = [];
        subdirComps[c.subdir].push(c);
    }
}

for (let i = 0; i < rootComps.length; i++) {
    const c = rootComps[i];
    wxs += `        <Component Id="C${i}" Guid="*">
          <File Id="F${i}" Name="${c.name}" Source="${c.source}" KeyPath="yes" />
        </Component>\n`;
}

for (const [sd, scomps] of Object.entries(subdirComps)) {
    const dirId = 'D' + sd.replace(/[^a-zA-Z0-9_]/g, '_');
    wxs += `        <Directory Id="${dirId}" Name="${sd}">\n`;
    for (const c of scomps) {
        const idx = components.indexOf(c);
        wxs += `          <Component Id="C${idx}" Guid="*">
            <File Id="F${idx}" Name="${c.name}" Source="${c.source}" KeyPath="yes" />
          </Component>\n`;
    }
    wxs += '        </Directory>\n';
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
console.log('Components: ' + components.length);