@echo off
setlocal
pushd "%~dp0" || exit /b 1

if not exist "build" mkdir "build"
del /q "build\com.moeilijk.lhm.streamDeckPlugin" 2>nul

DistributionTool.exe -b -i "com.moeilijk.lhm.sdPlugin" -o "build"

popd
endlocal
