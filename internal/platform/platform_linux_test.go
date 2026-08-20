//go:build linux

package platform

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePasswdHomes(t *testing.T) {
	passwd := `root:x:0:0:root:/root:/bin/bash
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
alice:x:1000:1000:Alice:/home/alice:/bin/bash
bob:x:1001:1001:Bob:/home/bob:/bin/zsh
service:x:999:999:Service:/var/run/service:/usr/sbin/nologin
nobody:x:65534:65534:nobody:/nonexistent:/usr/sbin/nologin
guest:x:65535:65535:guest:/tmp/guest:/bin/bash
malformed
# comment:x:1002:1002:ignored:/home/comment:/bin/bash
`

	homes := parsePasswdHomes(strings.NewReader(passwd))
	want := []string{"/home/alice", "/home/bob"}
	if len(homes) != len(want) {
		t.Fatalf("parsePasswdHomes: got %v, want %v", homes, want)
	}
	for i := range want {
		if homes[i] != want[i] {
			t.Errorf("parsePasswdHomes[%d]: got %q, want %q", i, homes[i], want[i])
		}
	}
}

func TestParsePasswdHomesEmpty(t *testing.T) {
	if homes := parsePasswdHomes(strings.NewReader("")); len(homes) != 0 {
		t.Errorf("empty passwd should yield no homes, got %v", homes)
	}
}

func TestParsePasswdHomesInvalidUID(t *testing.T) {
	passwd := `bad:x:notanumber:1000:Bad:/home/bad:/bin/bash
empty:x:1000:1000:Empty::/bin/bash
`
	homes := parsePasswdHomes(strings.NewReader(passwd))
	if len(homes) != 0 {
		t.Errorf("invalid/empty home should be skipped, got %v", homes)
	}
}

func TestUserHomesRootWithDedup(t *testing.T) {
	originalRoot := isRoot
	originalReader := passwdReader
	defer func() {
		isRoot = originalRoot
		passwdReader = originalReader
	}()
	isRoot = func() bool { return true }
	passwdReader = func() (io.Reader, error) {
		return strings.NewReader("alice:x:1000:1000:Alice:/home/alice:/bin/bash\nbob:x:1001:1001:Bob:/home/bob:/bin/zsh\n"), nil
	}

	homes := UserHomes("/home/current")
	want := []string{"/home/alice", "/home/bob", "/root"}
	if len(homes) != len(want) {
		t.Fatalf("root UserHomes: got %v, want %v", homes, want)
	}
	for i := range want {
		if homes[i] != want[i] {
			t.Errorf("UserHomes[%d]: got %q, want %q", i, homes[i], want[i])
		}
	}
}

func TestUserHomesRootDedupRootExists(t *testing.T) {
	originalRoot := isRoot
	originalReader := passwdReader
	defer func() {
		isRoot = originalRoot
		passwdReader = originalReader
	}()
	isRoot = func() bool { return true }
	passwdReader = func() (io.Reader, error) {
		return strings.NewReader("root2:x:1000:1000:root:/root:/bin/bash\n"), nil
	}

	homes := UserHomes("/home/current")
	if len(homes) != 1 || homes[0] != "/root" {
		t.Errorf("root home should be deduped, got %v", homes)
	}
}

func TestCoWTrue(t *testing.T) {
	orig := mountsReader
	defer func() { mountsReader = orig }()
	mountsReader = func() (io.Reader, error) {
		return strings.NewReader("device /mnt zfs rw 0 0\n"), nil
	}
	if !CoW() {
		t.Error("expected CoW for zfs")
	}
}

func TestCoWFalse(t *testing.T) {
	orig := mountsReader
	defer func() { mountsReader = orig }()
	mountsReader = func() (io.Reader, error) {
		return strings.NewReader("device / ext4 rw 0 0\ndevice2 /boot vfat ro 0 0\n"), nil
	}
	if CoW() {
		t.Error("expected no CoW for ext4/vfat")
	}
}

func TestCoWReaderError(t *testing.T) {
	orig := mountsReader
	defer func() { mountsReader = orig }()
	mountsReader = func() (io.Reader, error) { return nil, os.ErrNotExist }
	if CoW() {
		t.Error("expected false on reader error")
	}
}

func TestSSDTrue(t *testing.T) {
	orig := sysBlockDir
	defer func() { sysBlockDir = orig }()
	dir := t.TempDir()
	rot := filepath.Join(dir, "sda", "queue")
	if err := os.MkdirAll(rot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rot, "rotational"), []byte("0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sysBlockDir = dir
	if !SSD() {
		t.Error("expected SSD when rotational==0")
	}
}

func TestSSDFalse(t *testing.T) {
	orig := sysBlockDir
	defer func() { sysBlockDir = orig }()
	dir := t.TempDir()
	rot := filepath.Join(dir, "sda", "queue")
	if err := os.MkdirAll(rot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rot, "rotational"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sysBlockDir = dir
	if SSD() {
		t.Error("expected no SSD when rotational==1")
	}
}
