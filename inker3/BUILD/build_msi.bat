@echo off
setlocal enabledelayedexpansion

echo =============================================
echo  FtR Inker MSI Builder (WiX v7)
echo =============================================
echo.

set SCRIPT_DIR=%~dp0
set PROJECT_DIR=%SCRIPT_DIR%..
set OUTPUT_DIR=%SCRIPT_DIR%windows-x64
set APP_NAME=FtR Inker

echo Step 1: Check for WiX Toolset v7+...
where candle >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo ERROR: WiX Toolset (candle.exe) not found.
    echo Install WiX v7 from: https://wixtoolset.org/docs/wixv7/
    pause
    exit /b 1
)
echo [OK] WiX Toolset found.

echo.
echo Step 2: Check for electron-packager...
cd /d "%PROJECT_DIR%"
where npx >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo ERROR: npx not found. Install Node.js.
    pause
    exit /b 1
)
echo [OK] Node.js/npx found.

echo.
echo Step 3: Package application with electron-packager...
if exist "dist\%APP_NAME%-win32-x64" (
    echo Removing previous build...
    rmdir /s /q "dist\%APP_NAME%-win32-x64" >nul 2>nul
)

npx electron-packager . "%APP_NAME%" --platform=win32 --arch=x64 --out=dist --icon=icon.png --overwrite --no-prune 2>&1
if %ERRORLEVEL% neq 0 (
    echo ERROR: electron-packager failed.
    pause
    exit /b 1
)
echo [OK] Application packaged.

echo.
echo Step 4: Prepare output directory...
if exist "%OUTPUT_DIR%" rmdir /s /q "%OUTPUT_DIR%" >nul 2>nul
mkdir "%OUTPUT_DIR%" >nul 2>nul

set PACKAGED_DIR=%PROJECT_DIR%\dist\%APP_NAME%-win32-x64

echo.
echo Step 5: Generate WiX v7 source with auto-start registry entries...
set TEMP_WXS=%TEMP%\FtR_Inker_temp.wxs

REM Build WiX v7 WXS with:
REM - PerMachine install
REM - Start menu shortcut
REM - Desktop shortcut  
REM - Auto-start registry (HKCU\Software\Microsoft\Windows\CurrentVersion\Run)
REM - App icon
(
    echo ^<?xml version="1.0" encoding="UTF-8"?^>
    echo ^<Wix xmlns="http://wixtoolset.org/schemas/v7/wxs"^>
    echo   ^<Package Name="FtR Inker" Language="1033" Version="3.2.0" Manufacturer="FtR" UpgradeCode="5da499e7-668a-4df9-8a69-f1b6310fd334"^>
    echo     ^<MajorUpgrade DowngradeErrorMessage="A newer version of FtR Inker is already installed." /^>
    echo     ^<MediaTemplate EmbedCab="yes" /^>
    echo     ^<Feature Id="ProductFeature" Title="FtR Inker" Level="1"^>
    echo       ^<ComponentGroupRef Id="AppFiles" /^>
    echo       ^<ComponentRef Id="StartMenuShortcut" /^>
    echo       ^<ComponentRef Id="DesktopShortcut" /^>
    echo       ^<ComponentRef Id="AutoStart" /^>
    echo     ^</Feature^>
    echo     ^<StandardDirectory Id="ProgramFiles64Folder"^>
    echo       ^<Directory Id="INSTALLDIR" Name="FtR\Inker"^>
    echo         ^<ComponentGroup Id="AppFiles" Directory="INSTALLDIR"^>
    echo           ^<File Id="MainExe" Source="%PACKAGED_DIR:\=\\%\FtR Inker.exe" KeyPath="yes" /^>
    echo         ^</ComponentGroup^>
    echo       ^</Directory^>
    echo     ^</StandardDirectory^>
    echo     ^<StandardDirectory Id="ProgramMenuFolder"^>
    echo       ^<Directory Id="ApplicationProgramsFolder" Name="FtR Inker"^>
    echo         ^<Component Id="StartMenuShortcut" Guid="7a6bb7af-9f82-4e4f-8352-d6fa850a9fc3"^>
    echo           ^<Shortcut Id="StartMenuInker" Name="FtR Inker" Description="FtR Inker - InkDrop Client" Target="[INSTALLDIR]FtR Inker.exe" WorkingDirectory="INSTALLDIR" /^>
    echo           ^<RemoveFolder Id="CleanUpShortcut" Directory="ApplicationProgramsFolder" On="uninstall" /^>
    echo           ^<RegistryValue Root="HKCU" Key="Software\FtR\Inker" Name="Installed" Type="integer" Value="1" /^>
    echo         ^</Component^>
    echo       ^</Directory^>
    echo     ^</StandardDirectory^>
    echo     ^<StandardDirectory Id="DesktopFolder"^>
    echo       ^<Component Id="DesktopShortcut" Guid="60992b99-8521-4ba2-bf3d-f1de9d27f61c"^>
    echo         ^<Shortcut Id="DesktopInker" Name="FtR Inker" Description="FtR Inker - InkDrop Client" Target="[INSTALLDIR]FtR Inker.exe" WorkingDirectory="INSTALLDIR" /^>
    echo         ^<RegistryValue Root="HKCU" Key="Software\FtR\Inker" Name="DesktopShortcut" Type="integer" Value="1" /^>
    echo       ^</Component^>
    echo     ^</StandardDirectory^>
    echo     ^<Component Id="AutoStart" Guid="b3f17e2a-8d4c-4c5e-9f1a-2e7b8c9d0e1f"^>
    echo       ^<RegistryValue Root="HKCU" Key="Software\Microsoft\Windows\CurrentVersion\Run" Name="FtR Inker" Value="[INSTALLDIR]FtR Inker.exe" Type="string" /^>
    echo     ^</Component^>
    echo   ^</Package^>
    echo ^</Wix^>
) > "%TEMP_WXS%"

echo Generated WXS at %TEMP_WXS%

echo.
echo Step 6: Compile WiX source...
candle.exe "%TEMP_WXS%" -out "%TEMP%\FtR_Inker.wixobj" -nologo 2>&1
if %ERRORLEVEL% neq 0 (
    echo WiX v7 compilation failed. Trying WiX v5 fallback...

    REM Try with v5 namespace
    (
        echo ^<?xml version="1.0" encoding="UTF-8"?^>
        echo ^<Wix xmlns="http://wixtoolset.org/schemas/v5/wxs"^>
        echo   ^<Package Name="FtR Inker" Language="1033" Version="3.2.0" Manufacturer="FtR" UpgradeCode="5da499e7-668a-4df9-8a69-f1b6310fd334"^>
        echo     ^<MajorUpgrade DowngradeErrorMessage="A newer version of FtR Inker is already installed." /^>
        echo     ^<MediaTemplate EmbedCab="yes" /^>
        echo     ^<Feature Id="ProductFeature" Title="FtR Inker" Level="1"^>
        echo       ^<ComponentGroupRef Id="AppFiles" /^>
        echo       ^<ComponentRef Id="StartMenuShortcut" /^>
        echo       ^<ComponentRef Id="DesktopShortcut" /^>
        echo       ^<ComponentRef Id="AutoStart" /^>
        echo     ^</Feature^>
        echo     ^<StandardDirectory Id="ProgramFiles64Folder"^>
        echo       ^<Directory Id="INSTALLDIR" Name="FtR\Inker"^>
        echo         ^<ComponentGroup Id="AppFiles" Directory="INSTALLDIR"^>
        echo           ^<File Id="MainExe" Source="%PACKAGED_DIR:\=\\%\FtR Inker.exe" KeyPath="yes" /^>
        echo         ^</ComponentGroup^>
        echo       ^</Directory^>
        echo     ^</StandardDirectory^>
        echo     ^<StandardDirectory Id="ProgramMenuFolder"^>
        echo       ^<Directory Id="ApplicationProgramsFolder" Name="FtR Inker"^>
        echo         ^<Component Id="StartMenuShortcut" Guid="7a6bb7af-9f82-4e4f-8352-d6fa850a9fc3"^>
        echo           ^<Shortcut Id="StartMenuInker" Name="FtR Inker" Description="FtR Inker - InkDrop Client" Target="[INSTALLDIR]FtR Inker.exe" WorkingDirectory="INSTALLDIR" /^>
        echo           ^<RemoveFolder Id="CleanUpShortcut" Directory="ApplicationProgramsFolder" On="uninstall" /^>
        echo           ^<RegistryValue Root="HKCU" Key="Software\FtR\Inker" Name="Installed" Type="integer" Value="1" /^>
        echo         ^</Component^>
        echo       ^</Directory^>
        echo     ^</StandardDirectory^>
        echo     ^<StandardDirectory Id="DesktopFolder"^>
        echo       ^<Component Id="DesktopShortcut" Guid="60992b99-8521-4ba2-bf3d-f1de9d27f61c"^>
        echo         ^<Shortcut Id="DesktopInker" Name="FtR Inker" Description="FtR Inker - InkDrop Client" Target="[INSTALLDIR]FtR Inker.exe" WorkingDirectory="INSTALLDIR" /^>
        echo         ^<RegistryValue Root="HKCU" Key="Software\FtR\Inker" Name="DesktopShortcut" Type="integer" Value="1" /^>
        echo       ^</Component^>
        echo     ^</StandardDirectory^>
        echo     ^<Component Id="AutoStart" Guid="b3f17e2a-8d4c-4c5e-9f1a-2e7b8c9d0e1f"^>
        echo       ^<RegistryValue Root="HKCU" Key="Software\Microsoft\Windows\CurrentVersion\Run" Name="FtR Inker" Value="[INSTALLDIR]FtR Inker.exe" Type="string" /^>
        echo     ^</Component^>
        echo   ^</Package^>
        echo ^</Wix^>
    ) > "%TEMP_WXS%"

    candle.exe "%TEMP_WXS%" -out "%TEMP%\FtR_Inker.wixobj" -nologo 2>&1
    if %ERRORLEVEL% neq 0 (
        echo WiX v5 failed. Trying WiX v3...
        (
            echo ^<?xml version="1.0" encoding="UTF-8"?^>
            echo ^<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi"^>
            echo   ^<Product Id="*" Name="FtR Inker" Language="1033" Version="3.2.0" Manufacturer="FtR" UpgradeCode="5da499e7-668a-4df9-8a69-f1b6310fd334"^>
            echo     ^<Package InstallerVersion="500" Compressed="yes" InstallScope="perMachine" /^>
            echo     ^<MajorUpgrade DowngradeErrorMessage="A newer version of FtR Inker is already installed." /^>
            echo     ^<MediaTemplate EmbedCab="yes" /^>
            echo     ^<Feature Id="ProductFeature" Title="FtR Inker" Level="1"^>
            echo       ^<ComponentRef Id="AppDir" /^>
            echo       ^<ComponentRef Id="StartMenuShortcut" /^>
            echo       ^<ComponentRef Id="DesktopShortcut" /^>
            echo       ^<ComponentRef Id="AutoStart" /^>
            echo     ^</Feature^>
            echo     ^<Directory Id="TARGETDIR" Name="SourceDir"^>
            echo       ^<Directory Id="ProgramFiles64Folder"^>
            echo         ^<Directory Id="FtRFolder" Name="FtR"^>
            echo           ^<Directory Id="INSTALLDIR" Name="Inker"^>
            echo             ^<Component Id="AppDir" Guid="cdd3e292-e27f-4d19-8fcf-149ac97727d9"^>
            echo               ^<File Id="MainExe" Source="%PACKAGED_DIR:\=\\%\FtR Inker.exe" KeyPath="yes" /^>
            echo             ^</Component^>
            echo           ^</Directory^>
            echo         ^</Directory^>
            echo       ^</Directory^>
            echo       ^<Directory Id="ProgramMenuFolder"^>
            echo         ^<Directory Id="ApplicationProgramsFolder" Name="FtR Inker"^>
            echo           ^<Component Id="StartMenuShortcut" Guid="7a6bb7af-9f82-4e4f-8352-d6fa850a9fc3"^>
            echo             ^<Shortcut Id="StartMenuInker" Name="FtR Inker" Description="FtR Inker - InkDrop Client" Target="[INSTALLDIR]FtR Inker.exe" WorkingDirectory="INSTALLDIR" /^>
            echo             ^<RemoveFolder Id="CleanUpShortcut" Directory="ApplicationProgramsFolder" On="uninstall" /^>
            echo             ^<RegistryValue Root="HKCU" Key="Software\FtR\Inker" Name="Installed" Type="integer" Value="1" /^>
            echo           ^</Component^>
            echo         ^</Directory^>
            echo       ^</Directory^>
            echo       ^<Directory Id="DesktopFolder"^>
            echo         ^<Component Id="DesktopShortcut" Guid="60992b99-8521-4ba2-bf3d-f1de9d27f61c"^>
            echo           ^<Shortcut Id="DesktopInker" Name="FtR Inker" Description="FtR Inker - InkDrop Client" Target="[INSTALLDIR]FtR Inker.exe" WorkingDirectory="INSTALLDIR" /^>
            echo           ^<RegistryValue Root="HKCU" Key="Software\FtR\Inker" Name="DesktopShortcut" Type="integer" Value="1" /^>
            echo         ^</Component^>
            echo       ^</Directory^>
            echo     ^</Directory^>
            echo     ^<DirectoryRef Id="TARGETDIR"^>
            echo       ^<Component Id="AutoStart" Guid="b3f17e2a-8d4c-4c5e-9f1a-2e7b8c9d0e1f"^>
            echo         ^<RegistryValue Root="HKCU" Key="Software\Microsoft\Windows\CurrentVersion\Run" Name="FtR Inker" Value="[INSTALLDIR]FtR Inker.exe" Type="string" /^>
            echo       ^</Component^>
            echo     ^</DirectoryRef^>
            echo   ^</Product^>
            echo ^</Wix^>
        ) > "%TEMP_WXS%"
        candle.exe "%TEMP_WXS%" -out "%TEMP%\FtR_Inker.wixobj" -nologo 2>&1
        if %ERRORLEVEL% neq 0 (
            echo ERROR: WiX compilation failed with all namespaces (v7, v5, v3).
            pause
            exit /b 1
        )
    )
)

echo.
echo Step 7: Link MSI package...
light.exe "%TEMP%\FtR_Inker.wixobj" -out "%OUTPUT_DIR%\FtR-Inker-3.2.0-x64.msi" -nologo 2>&1
if %ERRORLEVEL% neq 0 (
    echo WARNING: Linking had some warnings, but continuing...
    if not exist "%OUTPUT_DIR%\FtR-Inker-3.2.0-x64.msi" (
        echo "ERROR: MSI was not created."
        pause
        exit /b 1
    )
)

echo.
echo =============================================
echo  SUCCESS!
echo =============================================
echo.
echo MSI built: %OUTPUT_DIR%\FtR-Inker-3.2.0-x64.msi
echo.
echo Features:
echo   - Per-machine install (Program Files)
echo   - Start menu shortcut
echo   - Desktop shortcut
echo   - Auto-start on boot (registry)
echo.
echo The MSI currently references the packaged app at:
echo   %PACKAGED_DIR%
echo.
echo For a fully standalone MSI, run heat.exe to harvest all files.
echo.
pause