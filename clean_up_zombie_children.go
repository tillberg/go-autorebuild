package autorebuild

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/tillberg/ansi-log"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

func CleanUpZombieChildren() {
	cmd := exec.Command("pgrep", "-P", fmt.Sprintf("%d", os.Getpid()))
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	err := cmd.Start()
	if err != nil {
		log.Println("Error starting pgrep in CleanUpZombieChildren", err)
	}
	err = cmd.Wait()
	if err != nil {
		_, ok := err.(*exec.ExitError)
		if !ok {
			log.Println("Failed to execute pgrep in CleanUpZombieChildren", err)
		}
	}
	go func() {
		scanner := bufio.NewScanner(&outBuf)
		for scanner.Scan() {
			line := scanner.Text()
			pid64, err := strconv.ParseInt(line, 10, 32)
			if err != nil {
				log.Println("Could not parse PID from line", line, "in CleanUpZombieChildren", err)
				continue
			}
			pid := int(pid64)
			if pid != cmd.Process.Pid {
				log.Printf("@(dim:Cleaning up zombie child) %d@(dim:.)\n", pid)
				go func() {
					syscall.Kill(pid, syscall.SIGHUP)
					time.Sleep(60 * time.Second)
					syscall.Kill(pid, syscall.SIGINT)
					time.Sleep(20 * time.Second)
					syscall.Kill(pid, syscall.SIGTERM)
				}()
				ws := syscall.WaitStatus(0)
				syscall.Wait4(pid, &ws, 0, nil)
			}
		}
	}()
}
