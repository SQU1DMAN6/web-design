INKER = "FtR Inker"

run:
	npm start

build:
	npx electron-packager . $(INKER) --platform=win32 --arch=x64 --out=dist --icon=icon.png --overwrite --no-prune

msi:
	npx electron-packager . $(INKER) --platform=win32 --arch=x64 --out=dist --icon=icon.png --overwrite --no-prune
	node BUILD\generate-wxs.js
	if not exist BUILD\windows-x64 mkdir BUILD\windows-x64
	wix build -arch x64 BUILD\FtR_Inker_full.wxs -out BUILD\windows-x64\FtR-Inker-3.2.0-x64.msi
	@echo MSI built: BUILD\windows-x64\FtR-Inker-3.2.0-x64.msi

check:
	node --check main.js && node --check preload.js && node --check renderer\renderer.js && node --check lib\api-client.js && node --check lib\sync-manager.js && node --check lib\settings.js && node --check lib\mount-manager.js

clean:
	if exist dist rmdir /s /q dist
	if exist BUILD\windows-x64 rmdir /s /q BUILD\windows-x64

.PHONY: run build msi check clean