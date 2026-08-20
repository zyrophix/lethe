//go:build linux

package platform

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var passwdReader = func() (io.Reader, error) {
	return os.Open("/etc/passwd")
}

var isRoot = func() bool {
	return os.Getuid() == 0
}

var mountsReader = func() (io.Reader, error) {
	return os.Open("/proc/self/mounts")
}

var sysBlockDir = "/sys/block"

var cowFilesystems = map[string]bool{
	"zfs":   true,
	"btrfs": true,
	"f2fs":  true,
	"ocfs2": true,
}

// ShredWarning returns a message when secure overwrite is ineffective on the
// current storage, or "" when shred works as advertised.
func ShredWarning() string {
	var reasons []string
	if SSD() {
		reasons = append(reasons, "SSD with wear-leveling")
	}
	if CoW() {
		reasons = append(reasons, "copy-on-write filesystem")
	}
	if len(reasons) == 0 {
		return ""
	}
	return "shred is ineffective on " + strings.Join(reasons, " and ")
}

// SSD reports whether any block device is non-rotational.
var SSD = func() bool {
	matches, err := filepath.Glob(filepath.Join(sysBlockDir, "*", "queue", "rotational"))
	if err != nil {
		return false
	}
	for _, m := range matches {
		data, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(data)) == "0" {
			return true
		}
	}
	return false
}

// CoW reports whether any mounted filesystem is copy-on-write.
var CoW = func() bool {
	r, err := mountsReader()
	if err != nil {
		return false
	}
	if c, ok := r.(io.Closer); ok {
		defer c.Close()
	}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		if cowFilesystems[fields[2]] {
			return true
		}
	}
	return false
}

// UserHomes returns home directories to clean. When run as root it returns all
// users from /etc/passwd (uid 1000..65534) plus /root, otherwise only the
// current user's home.
func UserHomes(homeDir string) []string {
	if !isRoot() {
		return []string{homeDir}
	}

	r, err := passwdReader()
	if err != nil {
		return []string{homeDir}
	}
	if c, ok := r.(io.Closer); ok {
		defer c.Close()
	}

	homes := parsePasswdHomes(r)
	homes = append(homes, "/root")
	return dedupeHomes(homes)
}

func parsePasswdHomes(r io.Reader) []string {
	var homes []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 7 {
			continue
		}
		uid, err := strconv.Atoi(strings.TrimSpace(parts[2]))
		if err != nil || uid < 1000 || uid >= 65534 {
			continue
		}
		home := strings.TrimSpace(parts[5])
		if home != "" {
			homes = append(homes, home)
		}
	}
	return homes
}

func detectLinux() Distro {
	d := Distro{Name: "linux", Version: "unknown", Family: "linux"}

	f, err := os.Open("/etc/os-release")
	if err != nil {
		if _, err := os.Stat("/etc/debian_version"); err == nil {
			d.Family = "debian"
		}
		if _, err := os.Stat("/etc/redhat-release"); err == nil {
			d.Family = "rhel"
		}
		return d
	}
	defer f.Close()

	var id, versionID string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "ID=") {
			id = strings.Trim(strings.TrimPrefix(line, "ID="), `"`)
		} else if strings.HasPrefix(line, "VERSION_ID=") {
			versionID = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), `"`)
		}
	}

	d.Name = id
	d.Version = versionID

	switch id {
	case "ubuntu", "debian", "pop", "linuxmint", "elementary", "kali":
		d.Family = "debian"
	case "rhel", "centos", "fedora", "rocky", "almalinux", "ol", "amzn":
		d.Family = "rhel"
	case "arch", "manjaro", "endeavouros", "garuda":
		d.Family = "arch"
	case "alpine":
		d.Family = "alpine"
	case "sles", "sled", "opensuse-leap", "opensuse-tumbleweed":
		d.Family = "suse"
	case "gentoo":
		d.Family = "gentoo"
	default:
		d.Family = "linux"
	}

	return d
}

func ApacheLogDir(d Distro) string {
	if d.IsRHEL() {
		return "/var/log/httpd"
	}
	return "/var/log/apache2"
}

func PackageCacheDir(d Distro) string {
	switch {
	case d.IsDebian():
		return "/var/cache/apt/archives"
	case d.IsRHEL():
		return "/var/cache/dnf"
	case d.IsArch():
		return "/var/cache/pacman/pkg"
	default:
		return ""
	}
}
