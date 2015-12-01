package archive

import (
	"github.com/Sirupsen/logrus"
	"os"
	"path/filepath"
)

// specifiesCurrentDir returns whether the given path specifies
// a "current directory", i.e., the last path segment is `.`.
func specifiesCurrentDir(path string) bool {
	return filepath.Base(path) == "."
}

// HasTrailingPathSeparator returns whether the given
// path ends with the system's path separator character.
func hasTrailingPathSeparator(path string) bool {
	return len(path) > 0 && os.IsPathSeparator(path[len(path)-1])
}

// PreserveTrailingDotOrSeparator returns the given cleaned path (after
// processing using any utility functions from the path or filepath stdlib
// packages) and appends a trailing `/.` or `/` if its corresponding  original
// path (from before being processed by utility functions from the path or
// filepath stdlib packages) ends with a trailing `/.` or `/`. If the cleaned
// path already ends in a `.` path segment, then another is not added. If the
// clean path already ends in a path separator, then another is not added.
func PreserveTrailingDotOrSeparator(cleanedPath, originalPath string) string {
	// Ensure paths are in platform semantics
	cleanedPath = normalizePath(cleanedPath)
	originalPath = normalizePath(originalPath)

	if !specifiesCurrentDir(cleanedPath) && specifiesCurrentDir(originalPath) {
		if !hasTrailingPathSeparator(cleanedPath) {
			// Add a separator if it doesn't already end with one (a cleaned
			// path would only end in a separator if it is the root).
			cleanedPath += string(filepath.Separator)
		}
		cleanedPath += "."
	}

	if !hasTrailingPathSeparator(cleanedPath) && hasTrailingPathSeparator(originalPath) {
		cleanedPath += string(filepath.Separator)
	}

	return cleanedPath
}

// TarResourceRebase is like TarResource but renames the first path element of
// items in the resulting tar archive to match the given rebaseName if not "".
func TarResourceRebase(sourcePath, rebaseName string) (content Archive, err error) {
	sourcePath = normalizePath(sourcePath)
	if _, err = os.Lstat(sourcePath); err != nil {
		// Catches the case where the source does not exist or is not a
		// directory if asserted to be a directory, as this also causes an
		// error.
		return
	}

	// Separate the source path between it's directory and
	// the entry in that directory which we are archiving.
	sourceDir, sourceBase := SplitPathDirEntry(sourcePath)

	filter := []string{sourceBase}

	logrus.Debugf("copying %q from %q", sourceBase, sourceDir)

	return TarWithOptions(sourceDir, &TarOptions{
		Compression:      Uncompressed,
		IncludeFiles:     filter,
		IncludeSourceDir: true,
		RebaseNames: map[string]string{
			sourceBase: rebaseName,
		},
	})
}

// SplitPathDirEntry splits the given path between its directory name and its
// basename by first cleaning the path but preserves a trailing "." if the
// original path specified the current directory.
func SplitPathDirEntry(path string) (dir, base string) {
	cleanedPath := filepath.Clean(normalizePath(path))

	if specifiesCurrentDir(path) {
		cleanedPath += string(filepath.Separator) + "."
	}

	return filepath.Dir(cleanedPath), filepath.Base(cleanedPath)
}
