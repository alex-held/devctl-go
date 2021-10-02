package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alex-held/devctl/pkg/env"
	"github.com/stretchr/testify/assert"
)

func tmpPath(t *testing.T, paths ...string) (root, install string) {
	basePath := t.TempDir()
	return basePath, filepath.Join(basePath, "sdks", "go", filepath.Join(paths...))
}

func TestHandleCurrent(t *testing.T) {
	const version = "v1.16.8"
	root, installPath := tmpPath(t)
	versionPath := filepath.Join(installPath, strings.TrimPrefix(version, "v"))
	currentPath := filepath.Join(installPath, "current")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	f := env.NewFactory(env.WithIO(nil, out, errOut), func(c *env.FactoryConfig) *env.FactoryConfig {
		c.Paths = env.NewPaths(root)
		return c
	})
	_ = os.MkdirAll(versionPath, os.ModePerm)
	_ = os.Symlink(versionPath, currentPath)

	err := handleCurrent(f)

	assert.NoError(t, err)
	assert.Equal(t, version, out.String())
	assert.Equal(t, "", errOut.String())
}

func TestHandleList(t *testing.T) {
	expected := "v1.13.5\nv1.16\nv1.16.3\nv1.16.4\nv1.16.8\nv1.17\nv1.17.1\n"
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root, installPath := tmpPath(t)
	currentPath := filepath.Join(installPath, "current")
	for _, version := range strings.Split(expected, "\n") {
		versionPath := filepath.Join(installPath, strings.TrimPrefix(version, "v"))
		_ = os.MkdirAll(versionPath, os.ModePerm)
		_ = os.Symlink(versionPath, currentPath)
	}
	f := env.NewFactory(env.WithIO(nil, out, errOut), func(c *env.FactoryConfig) *env.FactoryConfig {
		c.Paths = env.NewPaths(root)
		return c
	})

	err := handleList(f)
	assert.NoError(t, err)
	assert.Equal(t, expected, out.String())
}

func TestHandleInstall(t *testing.T) {
	const version = "v1.17.1"
	expected := ""
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root, installPath := tmpPath(t)
	versionPath := filepath.Join(installPath, strings.TrimPrefix(version, "v"))
	_ = os.MkdirAll(installPath, os.ModePerm)

	f := env.NewFactory(env.WithIO(nil, out, errOut), func(c *env.FactoryConfig) *env.FactoryConfig {
		c.Paths = env.NewPaths(root)
		return c
	})

	err := handleInstall(f, version)
	assert.NoError(t, err)
	assert.Equal(t, expected, out.String())
	assert.DirExists(t, versionPath)
}

func TestHandleUse(t *testing.T) {
	const version = "v1.17.1"
	expected := ""
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root, installPath := tmpPath(t)
	versionPath := filepath.Join(installPath, strings.TrimPrefix(version, "v"))
	currentPath := filepath.Join(installPath, "current")
	_ = os.MkdirAll(versionPath, os.ModePerm)

	f := env.NewFactory(env.WithIO(nil, out, errOut), func(c *env.FactoryConfig) *env.FactoryConfig {
		c.Paths = env.NewPaths(root)
		return c
	})

	err := handleUse(f, version)
	assert.NoError(t, err)

	linksTo, err := os.Readlink(currentPath)
	assert.Equal(t, versionPath, linksTo)
	assert.Equal(t, expected, out.String())
}
