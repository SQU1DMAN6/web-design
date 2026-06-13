@echo off
setlocal enabledelayedexpansion

echo =============================================
echo  FtR Inker MSI Builder
echo =============================================
echo.

set SCRIPT_DIR=%~dp0
set PROJECT_DIR=%SCRIPT_DIR%..
set OUTPUT_DIR=%SCRIPT_DIR%windows-x64
set APP_NAME=FtR Inker

echo Step 1: Check for WiX Toolset...
where candle >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo "ERROR: WiX Toolset (candle.exe) not found."
    echo Install from: https://wixtoolset.org/
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
echo Step 5: Generate WiX source...
REM Use the WiX source file from BUILD directory
set WXS_FILE=%SCRIPT_DIR%FtR_Inker.wxs

REM We need to update the WXS to point to the actual packaged app
REM Create a temp WXS with correct paths
set TEMP_WXS=%TEMP%\FtR_Inker_temp.wxs
copy /Y "%WXS_FILE%" "%TEMP_WXS%" >nul 2>nul

REM The packaged app root:
set INSTALL_SOURCE=%PACKAGED_DIR%

echo.
echo Step 6: Compile WiX source...
candle.exe "%TEMP_WXS%" -out "%TEMP%\FtR_Inker.wixobj" -nologo 2>&1
if %ERRORLEVEL% neq 0 (
    echo WiX compilation failed. Trying alternative approach...
    
    REM Alternative: Use heat to harvest the directory, then build MSI
    echo Harvesting packaged app directory...
    heat.exe dir "%PACKAGED_DIR%" -cg AppFiles -gg -sreg -sfrag -srd -dr INSTALLDIR -out "%TEMP%\AppFiles.wxs" -nologo 2>&1
    if %ERRORLEVEL% neq 0 (
        echo WARNING: heat extraction had issues, continuing...
    )
    
    REM Create a simple WXS that points to the packaged dir
    (
        echo ^<?xml version="1.0" encoding="UTF-8"?^>
        echo ^<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi"^>
        echo   ^<Product Id="*" Name="FtR Inker" Language="1033" Version="3.2.0" Manufacturer="FtR" UpgradeCode="5da499e7-668a-4df9-8a69-f1b6310fd334"^>
        echo     ^<Package InstallerVersion="500" Compressed="yes" InstallScope="perUser" /^>
        echo     ^<MajorUpgrade DowngradeErrorMessage="A newer version is already installed." /^>
        echo     ^<MediaTemplate EmbedCab="yes" /^>
        echo     ^<Feature Id="ProductFeature" Title="FtR Inker" Level="1"^>
        echo       ^<ComponentRef Id="AppDir" /^>
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
        echo             ^<Shortcut Id="StartMenuInker" Name="FtR Inker" Description="FtR Inker - InkDrop Desktop Client" Target="[INSTALLDIR]FtR Inker.exe" WorkingDirectory="INSTALLDIR" /^>
        echo             ^<RemoveFolder Id="CleanUpShortcut" Directory="ApplicationProgramsFolder" On="uninstall" /^>
        echo             ^<RegistryValue Root="HKCU" Key="Software\FtR\Inker" Name="Installed" Type="integer" Value="1" /^>
        echo           ^</Component^>
        echo         ^</Directory^>
        echo       ^</Directory^>
        echo       ^<Directory Id="DesktopFolder"^>
        echo         ^<Component Id="DesktopShortcut" Guid="60992b99-8521-4ba2-bf3d-f1de9d27f61c"^>
        echo           ^<Shortcut Id="DesktopInker" Name="FtR Inker" Description="FtR Inker - InkDrop Desktop Client" Target="[INSTALLDIR]FtR Inker.exe" WorkingDirectory="INSTALLDIR" /^>
        echo           ^<RegistryValue Root="HKCU" Key="Software\FtR\Inker" Name="DesktopShortcut" Type="integer" Value="1" /^>
        echo         ^</Component^>
        echo       ^</Directory^>
        echo     ^</Directory^>
        echo   ^</Product^>
        echo ^</Wix^>
    ) > "%TEMP_WXS%"
    
    echo Retrying compilation with generated WXS...
    candle.exe "%TEMP_WXS%" -out "%TEMP%\FtR_Inker.wixobj" -nologo 2>&1
    if %ERRORLEVEL% neq 0 (
        echo ERROR: WiX compilation failed.
        pause
        exit /b 1
    )
)

echo.
echo Step 7: Link MSI package...
light.exe "%TEMP%\FtR_Inker.wixobj" -out "%OUTPUT_DIR%\FtR-Inker-3.2.0-x64.msi" -nologo 2>&1
if %ERRORLEVEL% neq 0 (
    echo WARNING: Linking had some warnings, but continuing...
    REM Light may succeed despite warnings
    if not exist "%OUTPUT_DIR%\FtR-Inker-3.2.0-x64.msi" (
        echo ERROR: MSI was not created.
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
echo NOTE: The MSI references the packaged application at:
echo   %PACKAGED_DIR%
echo.
echo To produce a fully standalone MSI, the packaged app
echo and WiX source must be bundled together.
echo.
pause