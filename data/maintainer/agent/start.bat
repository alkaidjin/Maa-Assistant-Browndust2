@echo off
REM BD2MAA agent helper (dev/aux). Path-independent: resolves its own folder.
cd /d "%~dp0"
python "%~dp0start.py"
