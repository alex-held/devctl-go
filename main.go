package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/alex-held/devctl-kit/pkg/system"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/alex-held/devctl/pkg/cli/util"
	"github.com/alex-held/devctl/pkg/env"
)

var errOnlyOsFsSupported = errors.New("only afero.OsFs is supported")

func main() {
	cmd := NewCmd()
	util.CheckErr(cmd.Execute())
}

func NewCmd() *cobra.Command {
	f := env.NewFactory()
	cmd := &cobra.Command{
		Use:   "devctl-go",
		Short: "manages and installs go sdks",
		RunE: func(c *cobra.Command, args []string) error {
			return c.Help()
		},
	}

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "installs the provided version of the go sdk",
		RunE: func(c *cobra.Command, args []string) error {
			util.CheckErr(validateArgsForSubcommand("install", args, 1))
			return handleInstall(f, args[0])
		},
	}
	useCmd := &cobra.Command{
		Use:   "use",
		Short: "sets a go sdk version as the system default",
		RunE: func(cmd *cobra.Command, args []string) error {
			util.CheckErr(validateArgsForSubcommand("use", args, 1))
			return handleUse(f, args[0])
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "lists installed go sdks",
		RunE: func(c *cobra.Command, args []string) error {
			util.CheckErr(validateArgsForSubcommand("list", args, 0))
			return handleList(f)
		},
	}

	currentCmd := &cobra.Command{
		Use:   "current",
		Short: "prints the currently installed go version",
		RunE: func(c *cobra.Command, args []string) error {
			util.CheckErr(validateArgsForSubcommand("current", args, 0))
			return handleCurrent(f)
		},
	}

	cmd.AddCommand(currentCmd)
	cmd.AddCommand(listCmd)
	cmd.AddCommand(installCmd)
	cmd.AddCommand(useCmd)

	return cmd
}

func handleUse(f env.Factory, version string) error {
	base := filepath.Join(f.Paths().Base(), "sdks", "go")
	version = strings.TrimPrefix(version, "v")
	versionPath := path.Join(base, version)
	currentPath := filepath.Join(base, "current")

	osFs, ok := f.Fs().(*afero.OsFs)
	if !ok {
		return errOnlyOsFsSupported
	}

	_ = osFs.Remove(currentPath)
	if err := osFs.SymlinkIfPossible(versionPath, currentPath); err != nil {
		return err
	}
	return nil
}

func handleList(f env.Factory) error {
	installPath := filepath.Join(f.Paths().Base(), "sdks", "go")
	fis, err := afero.ReadDir(f.Fs(), installPath)
	if err != nil {
		return err
	}
	for _, fi := range fis {
		if fi.IsDir() {
			fmt.Fprintln(f.Streams().Out, fi.Name())
		}
	}
	return nil
}

func handleCurrent(f env.Factory) error {
	installPath := path.Join(filepath.Join(f.Paths().Base(), "sdks", "go", "current"))
	osFs, ok := f.Fs().(*afero.OsFs)
	if !ok {
		return errOnlyOsFsSupported
	}

	link, err := osFs.ReadlinkIfPossible(installPath)
	if err != nil {
		return err
	}

	currentDir := path.Base(link)
	currentVersion := "v" + currentDir
	fmt.Fprintf(f.Streams().Out, currentVersion)
	return nil
}

func handleInstall(f env.Factory, version string) error {
	version = strings.TrimPrefix(version, "v")
	installPath := path.Join(f.Paths().Base(), "sdks", "go", version)
	archive, err := dlArchive(version, f.Fs())
	if err != nil {
		return err
	}

	err = f.Fs().MkdirAll(installPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to Extract go sdk %s; dest=%s; archive=%s;err=%v\n", version, installPath, "*Bytes.Buffer", err)
	}
	err = unTarGzip(archive, installPath, unarchiveRenamer(), f.Fs())
	if err != nil {
		return fmt.Errorf("failed to Extract go sdk %s; dest=%s; archive=%s;err=%v\n", version, installPath, "*Bytes.Buffer", err)
	}
	return nil
}

func validateArgsForSubcommand(subcmd string, args []string, expected int) error {
	if len(args) != expected {
		return fmt.Errorf("provided wrong number of argument for subcommand '%s'; expected=%d; provided=%d", subcmd, expected, len(args))
	}
	return nil
}

func formatGoArchiveArtifactName(ri system.RuntimeInfo, version string) string {
	return ri.Format("go%s.[os]-[arch].tar.gz", version)
}

func dlArchive(version string, fs afero.Fs) (archive *bytes.Buffer, err error) {
	ri := system.OSRuntimeInfoGetter{}
	artifactName := formatGoArchiveArtifactName(ri.Get(), strings.TrimPrefix(version, "v"))
	dlUri := ri.Get().Format("%s/dl/%s", "https://golang.org", artifactName)

	buf := &bytes.Buffer{}
	err = download(context.Background(), dlUri, buf)
	if err != nil {
		return buf, fmt.Errorf("failed downloading go sdk %v from the remote server %s; err=%v", version, "https://golang.org", err)
	}

	return buf, nil
}

func download(ctx context.Context, url string, outWriter io.Writer) (err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(outWriter, resp.Body)
	return err
}

func unTarGzip(buf *bytes.Buffer, target string, renamer Renamer, fs afero.Fs) error {
	gr, _ := gzip.NewReader(buf)
	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()

		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		filename := header.Name
		if renamer != nil {
			filename = renamer(filename)
		}

		p := filepath.Join(target, filename)
		fi := header.FileInfo()

		if fi.IsDir() {
			if e := fs.MkdirAll(p, fi.Mode()); e != nil {
				return e
			}
			continue
		}
		file, err := fs.OpenFile(p, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fi.Mode())
		if err != nil {
			return err
		}

		_, err = io.Copy(file, tr)
		file.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

type Renamer func(p string) string

func unarchiveRenamer() Renamer {
	return func(p string) string {
		parts := strings.Split(p, string(filepath.Separator))
		parts = parts[1:]
		newPath := strings.Join(parts, string(filepath.Separator))
		return newPath
	}
}
