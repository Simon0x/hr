package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func cmdDefault(root string) int {
	addr := stackAddr()
	if stackHealthy(addr) {
		return attachWorkerOnly(root, addr)
	}

	if os.Getenv("HR_SERVER_DB") == "" {
		if _, err := os.Stat(filepath.Join(root, "docker-compose.yml")); err != nil {
			fmt.Fprintf(os.Stderr, "no hr reachable at %s, and no docker-compose.yml here to start postgres for one - set HR_SERVER_ADDR to point at your team's instance, or HR_SERVER_DB at a database\n", addr)
			return 0
		}
	}

	return startUnified(root, addr)
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		if os.Getenv("WSL_DISTRO_NAME") != "" {
			return exec.Command("cmd.exe", "/c", "start", "", url).Start()
		}
		return exec.Command("xdg-open", url).Start()
	}
}
