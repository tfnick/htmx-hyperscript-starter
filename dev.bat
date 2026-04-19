@echo off
REM dev.bat - Windows Start Script
REM useage: dev.bat [port]
REM demo: dev.bat 3000

set PORT=%1
if "%PORT%"=="" set PORT=3000

echo Starting development server on port %PORT%...
echo.

go run . --dev --port %PORT%
