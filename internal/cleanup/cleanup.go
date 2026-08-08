// Package cleanup plans and removes ignored repository content.
package cleanup

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Target is an ignored file or directory selected for removal.
type Target struct {
	Path  string
	Size  int64
	Files int
	Dirs  int
	IsDir bool
}

// Plan validates git-reported relative paths and calculates the size of the
// ignored content they identify. Paths below a selected directory are omitted,
// because removing the directory removes them too.
func Plan(root string, paths []string) ([]Target, error) {
	seen := make(map[string]bool, len(paths))
	var targets []Target
	for _, path := range paths {
		path = strings.TrimSuffix(path, "/")
		if path == "" || filepath.IsAbs(path) || path == "." || path == ".." || strings.HasPrefix(filepath.Clean(path), ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("unsafe ignored path %q", path)
		}
		path = filepath.Clean(path)
		if seen[path] {
			continue
		}
		seen[path] = true
		info, err := os.Lstat(filepath.Join(root, path))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", path, err)
		}
		size, files, dirs, err := pathSize(filepath.Join(root, path), info)
		if err != nil {
			return nil, err
		}
		targets = append(targets, Target{Path: path, Size: size, Files: files, Dirs: dirs, IsDir: info.IsDir()})
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].Path < targets[j].Path })

	filtered := targets[:0]
	for _, target := range targets {
		contained := false
		for _, parent := range filtered {
			if parent.IsDir && strings.HasPrefix(target.Path, parent.Path+string(filepath.Separator)) {
				contained = true
				break
			}
		}
		if !contained {
			filtered = append(filtered, target)
		}
	}
	return filtered, nil
}

func pathSize(path string, info fs.FileInfo) (int64, int, int, error) {
	if !info.IsDir() {
		return info.Size(), 1, 0, nil
	}
	var size int64
	files, dirs := 0, 1
	err := filepath.WalkDir(path, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if current != path {
				dirs++
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		size += info.Size()
		files++
		return nil
	})
	if err != nil {
		return 0, 0, 0, fmt.Errorf("measure %s: %w", path, err)
	}
	return size, files, dirs, nil
}

// Remove deletes exactly one target beneath root.
func Remove(root string, target Target) error {
	path := filepath.Join(root, target.Path)
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove %s: %w", target.Path, err)
	}
	return nil
}

// PruneEmptyParents removes now-empty ancestor directories of target, stopping
// at root. It never removes a directory that still contains content.
func PruneEmptyParents(root string, target Target) ([]string, error) {
	root = filepath.Clean(root)
	dir := filepath.Dir(filepath.Join(root, target.Path))
	var removed []string
	for dir != root {
		err := os.Remove(dir)
		if err == nil {
			rel, relErr := filepath.Rel(root, dir)
			if relErr != nil {
				return nil, relErr
			}
			removed = append(removed, rel)
			dir = filepath.Dir(dir)
			continue
		}
		if os.IsNotExist(err) || strings.Contains(err.Error(), "not empty") {
			break
		}
		return nil, fmt.Errorf("remove empty directory %s: %w", dir, err)
	}
	return removed, nil
}

// TotalSize returns the sum of target sizes.
func TotalSize(targets []Target) int64 {
	var size int64
	for _, target := range targets {
		size += target.Size
	}
	return size
}

// Counts returns the number of files and directories represented by targets.
func Counts(targets []Target) (files, dirs int) {
	for _, target := range targets {
		files += target.Files
		dirs += target.Dirs
	}
	return files, dirs
}
