package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
)

type ErrNoConfigsFound struct {
	Dir string
}

func (e ErrNoConfigsFound) Error() string {
	return fmt.Sprintf("No Vulcan config files found in: %s", e.Dir)
}

func LoadDir(root string) (*Config, error) {
	files, err := listFiles(root)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, &ErrNoConfigsFound{Dir: root}
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	var result *Config

	sort.Strings(files)

	for _, file := range files {
		c, err := LoadFile(file)
		if err != nil {
			return nil, err
		}

		if result != nil {
			result, err = Append(result, c)
			if err != nil {
				return nil, err
			}
		} else {
			result = c
		}
	}

	result.Dir = rootAbs

	return result, nil
}

func LoadFile(path string) (*Config, error) {
	c, err := loadFileHcl(path)
	if err != nil {
		return nil, err
	}

	return c.Config()
}

func listFiles(dir string) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	if !fi.IsDir() {
		return nil, fmt.Errorf("configuration path must be a directory: %s", dir)
	}

	var files []string
	err = nil
	for err != io.EOF {
		var fis []os.FileInfo
		fis, err = f.Readdir(128)
		if err != nil && err != io.EOF {
			return nil, err
		}

		log.Debugf("Found %d files", len(fis))

		for _, fi := range fis {
			log.Debugf("Found: %s", fi.Name())
			// ignore directories for now
			if fi.IsDir() {
				continue
			}

			// Only care about correct extensions
			name := fi.Name()
			if !strings.HasSuffix(name, ".hcl") {
				log.Debugf("Skipping: %s", name)
				continue
			}

			files = append(files, filepath.Join(dir, name))
		}
	}

	return files, nil
}
