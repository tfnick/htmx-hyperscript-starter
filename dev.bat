@echo off
setlocal

cd /d "%~dp0"

where gow >nul 2>nul
if %errorlevel% equ 0 (
  gow -e=go run . --dev --template-path public --static-path public
) else (
  echo gow was not found on PATH; running without Go file hot reload.
  go run . --dev --template-path public --static-path public
)

exit /b %errorlevel%
