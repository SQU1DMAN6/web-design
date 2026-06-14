@echo off
setlocal enabledelayedexpansion

echo =============================================
echo  FtR Inker MSI Builder (WiX v7)
echo =============================================
echo.

set SCRIPT_DIR=%~dp0
set PROJECT_DIR=%SCRIPT_DIR%..
set APP_DIR=%PROJECT_DIR%\dist\FtR Inker-win32-x64
set OUTPUT_DIR=%SCRIPT_DIR%windows-x64
set WXS_FILE=%SCRIPT_DIR%FtR_Inker_v7.wxs

echo Step 1: Check for WiX Toolset v7...
where wix >nul 2>nul
if errorlevel 1 (
    echo ERROR: wix.exe not found. Install WiX v7 from https://wixtoolset.org/
    pause
    exit /b 1
)
wix --version
echo [OK] WiX Toolset found.

echo.
echo Step 2: Check portable app...
if not exist "%APP_DIR%\FtR Inker.exe" (
    echo ERROR: Portable app not found at %APP_DIR%
    echo Run 'make build' first or 'pnpm exec electron-builder --win'
    pause
    exit /b 1
)
echo [OK] App found

echo.
echo Step 3: Create output directory...
if exist "%OUTPUT_DIR%" rmdir /s /q "%OUTPUT_DIR%" 2>nul
mkdir "%OUTPUT_DIR%" 2>nul
set MSI_OUTPUT=%OUTPUT_DIR%\FtR-Inker-3.2.0-x64.msi

echo.
echo Step 4: Generate complete WiX source with all files...
echo.
node "%SCRIPT_DIR%generate-wxs.js"
if errorlevel 1 (
    echo ERROR: Could not generate WXS.
    pause
    exit /b 1
)

echo.
echo Step 5: Build MSI with WiX v7...
echo Source: %SCRIPT_DIR%FtR_Inker_full.wxs
echo Output: %MSI_OUTPUT%
echo.

wix build -arch x64 "%SCRIPT_DIR%FtR_Inker_full.wxs" -out "%MSI_OUTPUT%" -intermediatefolder "%TEMP%\wix_build"
if errorlevel 1 (
    echo ERROR: WiX build failed.
    pause
    exit /b 1
)

echo.
if not exist "%MSI_OUTPUT%" (
    echo ERROR: MSI was not created.
    pause
    exit /b 1
)

echo =============================================
echo  SUCCESS!
echo =============================================
echo.
echo MSI built: %MSI_OUTPUT%
dir "%MSI_OUTPUT%"
echo.
echo Features:
echo   - Full Electron runtime bundled (standalone)
echo   - Program Files\FtR\Inker install
echo   - Start menu + Desktop shortcuts
echo   - Auto-start on boot (registry)
echo.
pause