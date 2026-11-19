--- internal/app/cmd/daemon.go.orig	2026-11-19 05:17:22 UTC
+++ internal/app/cmd/daemon.go
@@ -371,6 +371,7 @@ var commonSocketPaths = []string{
 var commonSocketPaths = []string{
 	"/var/run/docker.sock",
 	"/run/podman/podman.sock",
+	"/var/run/podman/podman.sock",
 	"$HOME/.colima/docker.sock",
 	"$XDG_RUNTIME_DIR/docker.sock",
 	"$XDG_RUNTIME_DIR/podman/podman.sock",
