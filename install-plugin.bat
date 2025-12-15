@echo off
setlocal
pushd "%~dp0" || exit /b 1

call ".\kill-streamdeck.bat"

xcopy "com.extension.lhm.sdPlugin" "%APPDATA%\Elgato\StreamDeck\Plugins\com.extension.lhm.sdPlugin\" /E /I /Q /Y

call ".\start-streamdeck.bat"

popd
endlocal
