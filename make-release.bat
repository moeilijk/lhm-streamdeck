@echo off
setlocal
pushd "%~dp0" || exit /b 1

if not exist "build" mkdir "build"
del /q "build\com.moeilijk.lhm.streamDeckPlugin" 2>nul

powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\\bump-manifest-version.ps1"
if errorlevel 1 exit /b 1

call streamdeck validate "com.moeilijk.lhm.sdPlugin"
if errorlevel 1 exit /b 1
call streamdeck pack "com.moeilijk.lhm.sdPlugin" --output "build" --force

popd
endlocal
