run:
	npm start

build:
	npm install
	cd native && npm install && npx node-gyp rebuild
	Copy-Item native\build\Release\winfsp_native.node lib\winfsp_native.node -Force
	npx electron-builder --config electron-builder.yml

msi:
	npm install
	cd native && npm install && npx node-gyp rebuild
	Copy-Item native\build\Release\winfsp_native.node lib\winfsp_native.node -Force
	npx electron-packager . --platform=win32 --arch=x64 --out=dist --icon=icon.png --overwrite
	node BUILD\generate-wxs.js
	if (-not (Test-Path BUILD\windows-x64)) { New-Item -ItemType Directory -Path BUILD\windows-x64 }
	wix build -arch x64 BUILD\FtR_Inker_full.wxs -out BUILD\windows-x64\FtR-Inker-3.2.0-x64.msi
	@echo MSI built: BUILD\windows-x64\FtR-Inker-3.2.0-x64.msi

check:
	node --check main.js
	node --check preload.js
	node --check renderer/renderer.js
	node --check lib/api-client-v2.js
	node --check lib/settings.js
	node --check lib/cache.js
	node --check lib/hydrator.js
	node --check lib/virtual-fs.js
	node --check lib/winfsp-bridge.js

clean:
	if (Test-Path lib/winfsp_native.node) { Remove-Item lib/winfsp_native.node -Force }
	if (Test-Path native/build) { Remove-Item native/build -Recurse -Force }
	if (Test-Path native/node_modules) { Remove-Item native/node_modules -Recurse -Force }
	if (Test-Path node_modules) { Remove-Item node_modules -Recurse -Force }
	if (Test-Path dist) { Remove-Item dist -Recurse -Force }
	if (Test-Path BUILD/windows-x64) { Remove-Item BUILD/windows-x64 -Recurse -Force }
	Write-Host "Clean complete. Run 'make' to rebuild."

rebuild: clean
	npm install
	cd native && npm install && npx node-gyp rebuild
	Copy-Item native\build\Release\winfsp_native.node lib\winfsp_native.node -Force
	Write-Host "Rebuild complete. Run 'make run' to start."

.PHONY: run build msi check clean rebuild