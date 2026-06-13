run:
	npm start

build:
	npx electron-packager . "FtR Inker" --platform=win32 --arch=x64 --out=dist --icon=icon.png --overwrite --no-prune

msi:
	cd BUILD && build_msi.bat

check:
	node --check main.js && node --check preload.js && node --check renderer/renderer.js && node --check lib/api-client.js && node --check lib/sync-manager.js && node --check lib/settings.js && node --check lib/mount-manager.js

clean:
	if exist dist rmdir /s /q dist

.PHONY: run build msi check clean