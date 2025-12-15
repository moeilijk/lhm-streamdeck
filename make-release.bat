@echo off
setlocal
pushd "%~dp0" || exit /b 1

if not exist "build" mkdir "build"
del /q "build\com.extension.lhm.streamDeckPlugin" 2>nul

DistributionTool.exe -b -i "com.extension.lhm.sdPlugin" -o "build"

popd
endlocal
