@echo off
echo Building FtR Inker MSI...
echo.

REM Check for WiX Toolset
where candle >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo Error: WiX Toolset (candle.exe) not found.
    echo Install WiX Toolset from: https://wixtoolset.org/
    echo.
    pause
    exit /b 1
)

echo Found WiX Toolset.
echo Compiling WiX source...
candle.exe FtR_Inker.wxs -out FtR_Inker.wixobj
if %ERRORLEVEL% neq 0 (
    echo Failed to compile WiX source.
    pause
    exit /b 1
)

echo Linking MSI package...
light.exe FtR_Inker.wixobj -out "..\FtR_Inker-3.2.0-x64.msi"
if %ERRORLEVEL% neq 0 (
    echo Failed to link MSI package.
    pause
    exit /b 1
)

echo.
echo MSI built successfully: ..\FtR_Inker-3.2.0-x64.msi
pause