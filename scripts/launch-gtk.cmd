@echo off
setlocal
cd /d "%~dp0"
set "PATH=%~dp0;%PATH%"
set "XDG_DATA_DIRS=%~dp0share"
set "GSETTINGS_SCHEMA_DIR=%~dp0share\glib-2.0\schemas"
set "GDK_PIXBUF_MODULEDIR=%~dp0lib\gdk-pixbuf-2.0\2.10.0\loaders"
set "GDK_PIXBUF_MODULE_FILE=%TEMP%\kapila-pixbuf-%RANDOM%-%RANDOM%.cache"
"%~dp0gdk-pixbuf-query-loaders.exe" > "%GDK_PIXBUF_MODULE_FILE%"
if errorlevel 1 exit /b 1
"%~dp0kapila-burner.exe" %*
set "result=%ERRORLEVEL%"
del "%GDK_PIXBUF_MODULE_FILE%"
exit /b %result%
