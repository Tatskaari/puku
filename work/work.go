package work

import (
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/please-build/buildtools/labels"

	"github.com/please-build/puku/config"
)

func MustExpandPaths(origWD string, paths []string) []string {
	paths, err := ExpandPaths(origWD, paths)
	if err != nil {
		log.Fatalf("failed to expands paths: %v", err)
	}
	return paths
}

// ExpandPaths expands the paths passed in by the user, resolving any `...` wildcards to the subdirectories of the path
// passed in. The paths are made relative to the repo root. By this point, if we were in a subdirectory of the repo,
// puku will have changed its working directory to the repo root, but recorded the original working directory. The
// original working directory is passed in here, so we can join any relative paths with that directory to make them
// relative to the repo root.
func ExpandPaths(originalWorkingDir string, paths []string) ([]string, error) {
	if len(paths) == 0 {
		return ExpandPaths(originalWorkingDir, []string{"..."})
	}
	// We assume by this point, we have changed directory to the repo root.
	repoRoot, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	ret := make([]string, 0, len(paths))
	for _, path := range paths {
		// Handle using build label style syntax a bit like `plz build`
		if strings.HasPrefix(path, "//") {
			l := labels.Parse(path)
			path = l.Package
		} else {
			if strings.HasPrefix(path, ":") {
				path = "."
			}
		}

		isWildcard := false
		if filepath.Base(path) == "..." {
			isWildcard = true
			path = filepath.Dir(path)
		}

		path = filepath.Clean(path)
		if filepath.IsAbs(path) {
			p, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return nil, err
			}
			path = p
		} else {
			path = filepath.Join(originalWorkingDir, path)
		}

		if !isWildcard {
			// This allows passing the file that changed or the BUILD file instead of the directory
			if stat, err := os.Lstat(path); err == nil && !stat.IsDir() {
				path = filepath.Dir(path)
			}
			ret = append(ret, path)
			continue
		}

		err = filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() {
				return nil
			}
			if d.Name() == "plz-out" {
				return filepath.SkipDir
			}
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			conf, err := config.ReadConfig(path)
			if err != nil {
				return err
			}

			if conf.GetStop() {
				return filepath.SkipDir
			}
			ret = append(ret, path)
			return nil
		})

		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

// FindRoot finds the root of the workspace
func FindRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return findRoot(dir)
}

func findRoot(path string) (string, error) {
	if path == "." {
		return "", errors.New("failed to locate please repo root: no .plzconfig found")
	}
	info, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}

	for _, i := range info {
		if i.IsDir() {
			continue
		}
		if i.Name() == ".plzconfig" {
			return path, nil
		}
	}
	return findRoot(filepath.Dir(path))
}
