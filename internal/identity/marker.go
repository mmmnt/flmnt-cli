package identity

import (
	"errors"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

// Marker reads the host machine's identifying marker: macOS IOPlatformUUID, Linux machine-id, Windows
// MachineGuid. A machine that yields none is refused — an identity is never invented.
func Marker() (string, error) {
	var marker string
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
		if err != nil {
			return "", err
		}
		marker = parseIORegUUID(string(out))
	case "windows":
		out, err := exec.Command("reg", "query", `HKLM\SOFTWARE\Microsoft\Cryptography`, "/v", "MachineGuid").Output()
		if err != nil {
			return "", err
		}
		marker = parseRegQuery(string(out))
	default:
		for _, path := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
			if raw, err := os.ReadFile(path); err == nil {
				marker = strings.TrimSpace(string(raw))
				break
			}
		}
	}
	if marker == "" {
		return "", errors.New("no machine marker could be read from this host")
	}
	return marker, nil
}

var ioregUUID = regexp.MustCompile(`"IOPlatformUUID"\s*=\s*"([^"]+)"`)

func parseIORegUUID(out string) string {
	if m := ioregUUID.FindStringSubmatch(out); m != nil {
		return m[1]
	}
	return ""
}

func parseRegQuery(out string) string {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "MachineGuid" {
			return fields[len(fields)-1]
		}
	}
	return ""
}
