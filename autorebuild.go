package autorebuild

import (
	"github.com/howeyc/fsnotify"
	"github.com/tillberg/ansi-log"
	"github.com/tillberg/bismuth"
	"os"
	"path"
	"path/filepath"
	"syscall"
)

func watchForChanges(watchRoot string) *fsnotify.Watcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Bail(err)
	}
	go filepath.Walk(watchRoot, func(p string, info os.FileInfo, err error) error {
		base := filepath.Base(p)
		if base == ".git" || base == "node_modules" {
			return filepath.SkipDir
		}
		if info != nil && info.IsDir() {
			watcher.Watch(p)
		}
		return nil
	})
	return watcher
}

const logPrefix = "@(dim:[autorebuild]) "

func RestartOnChange(srcPath string) {
	exeName := path.Base(os.Args[0])
	autorebuildTemp := path.Join(os.TempDir(), "go-autorebuild")
	buildPath := path.Join(autorebuildTemp, exeName)
	exePath := path.Join(buildPath, exeName)
	ctx := bismuth.NewExecContext()
	ctx.Connect()
	watcher := watchForChanges(srcPath)
	for {
		ctx.Quote("autorebuild/cleanup", "rm", "-rf", buildPath)
		ctx.Run("rmdir", autorebuildTemp) // Clean the directory up iff empty

		select {
		case ev := <-watcher.Event:
			p, err := filepath.Rel(srcPath, ev.Name)
			if err != nil {
				p = ev.Name
			}
			log.Printf(logPrefix+"%s @(dim:changed. Rebuilding...)\n", p)
		case err := <-watcher.Error:
			log.Printf(logPrefix+"watcher error: %s\n", err)
			continue
		}

		ctx.Mkdirp(buildPath)
		retCode, err := ctx.Quote("autorebuild/rsync", "rsync", "-a", path.Clean(srcPath)+"/", path.Clean(buildPath)+"/")
		if retCode != 0 {
			log.Printf(logPrefix+"rsync returned non-zero %d\n", retCode)
			continue
		}
		if err != nil {
			log.Printf(logPrefix+"rsync errored: %v\n", err)
			continue
		}
		retCode, err = ctx.QuoteCwd("autorebuild/go-build", buildPath, "/home/ubuntu/go/bin/go", "build")
		if retCode != 0 {
			log.Printf(logPrefix + "@(red:Build failed.)\n")
			continue
		}
		if err != nil {
			log.Printf(logPrefix+"go-build errored: %v\n", err)
			continue
		}
		fileInfo, err := os.Stat(exePath)
		if err != nil {
			log.Printf(logPrefix+"stat errored: %v\n", err)
			continue
		}
		log.Printf(logPrefix+"@(green:%s build successful, %dkb. Restarting...)\n", exeName, fileInfo.Size()/1024)
		syscall.Exec(exePath, os.Args, os.Environ())
	}
}
