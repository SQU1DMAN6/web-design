run:
	@if not exist lib\winfsp_native.node (cd native && npm install && npx node-gyp rebuild && copy /y build\Release\winfsp_native.node ..\lib\winfsp_native.node)
	npm start

setup:
	npm install
	cd native && npm install && npx node-gyp rebuild
	copy /y native\build\Release\winfsp_native.node lib\winfsp_native.node
	@echo [Maker] Setup complete. Run 'make run' to start the application.

build:
	npm install
	cd native && npm install && npx node-gyp rebuild
	copy /y native\build\Release\winfsp_native.node lib\winfsp_native.node
	npx electron-builder --config electron-builder.yml

msi:
	npm install
	cd native && npm install && npx node-gyp rebuild
	copy /y native\build\Release\winfsp_native.node lib\winfsp_native.node
	npx electron-packager . --platform=win32 --arch=x64 --out=dist --icon=icon.png --overwrite
	node BUILD\generate-wxs.js
	if not exist BUILD\windows-x64 mkdir BUILD\windows-x64
	wix build -arch x64 BUILD\FtR_Inker_full.wxs -out BUILD\windows-x64\FtR-Inker-3.2.0-x64.msi
	@echo [Maker] MSI built.

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
	@if exist lib\winfsp_native.node del /f /q lib\winfsp_native.node
	@if exist native\build rmdir /s /q native\build
	@if exist native\node_modules rmdir /s /q native\node_modules
	@if exist node_modules rmdir /s /q node_modules
	@if exist dist rmdir /s /q dist
	@if exist BUILD\windows-x64 rmdir /s /q BUILD\windows-x64
	@echo [Maker] Clean complete.

rebuild: clean setup

.PHONY: run setup build msi check clean rebuild