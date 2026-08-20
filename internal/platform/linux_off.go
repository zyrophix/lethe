//go:build !linux

package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

func detectLinux() Distro {
	return Distro{}
}

func ApacheLogDir(d Distro) string {
	return ""
}

func PackageCacheDir(d Distro) string {
	return ""
}

var isRoot = func() bool {
	return os.Getuid() == 0
}

// ShredWarning returns a message when secure overwrite is ineffective on the
// current storage, or "" when shred works as advertised. Non-Linux returns "".
func ShredWarning() string {
	return ""
}

var SSD = func() bool {
	return false
}

var CoW = func() bool {
	return false
}

// UserHomes returns home directories to clean. When run as root it globs
// /Users/* (excluding Shared/Guest/Public/Default), otherwise only the
// current user's home.
func UserHomes(homeDir string) []string {
	if !isRoot() {
		return []string{homeDir}
	}

	var base string
	switch runtime.GOOS {
	case "darwin":
		base = "/Users"
	default:
		base = filepath.Join(os.Getenv("SystemDrive"), `\Users`)
	}

	matches, err := filepath.Glob(filepath.Join(base, "*"))
	if err != nil {
		return []string{homeDir}
	}

	var homes []string
	for _, m := range matches {
		name := filepath.Base(m)
		if name == "Shared" || name == "Guest" || name == "Public" || name == "Default" {
			continue
		}
		homes = append(homes, m)
	}
	return dedupeHomes(append(homes, homeDir))
}
