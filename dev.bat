@echo off
setlocal

cd /d "%~dp0"

where gow >nul 2>nul
if %errorlevel% equ 0 (
  gow -e=go run . --dev
) else (
  echo gow was not found on PATH; running without Go file hot reload.
  go run . --dev
)

exit /b %errorlevel%
