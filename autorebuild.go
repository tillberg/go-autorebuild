package autorebuild

import (
	"github.com/tillberg/bismuth"
	"os"
	"path"
	"syscall"
)

func inotifyArgs() []string {
	args := []string{
		"inotifywait",
		"--quiet",
		"--recursive",
	}
	inotifyChangeEvents := []string{"modify", "attrib", "move", "create", "delete"}
	for _, event := range inotifyChangeEvents {
		args = append(args, "--event", event)
	}
	args = append(args, ".")
	return args
}

func RestartOnChange(srcPath string) {
	exeName := path.Base(os.Args[0])
	autorebuildTemp := path.Join(os.TempDir(), "go-autorebuild")
	buildPath := path.Join(autorebuildTemp, exeName)
	exePath := path.Join(buildPath, exeName)
	ctx := bismuth.NewExecContext()
	ctx.Connect()
	logger := ctx.Logger()
	for {
		ctx.Quote("autorebuild/cleanup", "rm", "-rf", buildPath)
		ctx.Run("rmdir", autorebuildTemp) // Clean the directory up iff empty

		logger.Printf("@(dim:Watching for changes...)\n")
		ctx.RunCwd(ctx.AbsPath(srcPath), inotifyArgs()...)

		logger.Printf("@(dim:Source change detected. Rebuilding...)\n")
		ctx.Mkdirp(buildPath)
		retCode, err := ctx.Quote("autorebuild/rsync", "rsync", "-a", srcPath, buildPath)
		if retCode != 0 {
			logger.Printf("rsync returned non-zero %d\n", retCode)
			continue
		}
		if err != nil {
			logger.Printf("rsync errored: %v\n", err)
			continue
		}
		retCode, err = ctx.QuoteCwd("autorebuild/go-build", buildPath, "/home/ubuntu/go/bin/go", "build")
		if retCode != 0 {
			logger.Printf("@(red:Build failed.)\n")
			continue
		}
		if err != nil {
			logger.Printf("go-build errored: %v\n", err)
			continue
		}
		fileInfo, err := os.Stat(exePath)
		if err != nil {
			logger.Printf("stat errored: %v\n", err)
			continue
		}
		logger.Printf("@(green:Build successful, %dkb. Restarting with new build.)\n", fileInfo.Size()/1024)
		syscall.Exec(exePath, os.Args, os.Environ())
	}
}
