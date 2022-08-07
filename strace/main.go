package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"

	seccomp "github.com/seccomp/libseccomp-golang"
)

func main() {
	fmt.Printf(">>>running %s with args %s\n", os.Args[1], os.Args[2:])

	syscallCounter := map[string]int{}

	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Ptrace: true,
	}

	cmd.Run()
	pid := cmd.Process.Pid

	stage := "enter"

	for {

		var regs syscall.PtraceRegs
		err := syscall.PtraceGetRegs(pid, &regs)

		if err != nil {
			if err.Error() == "no such process" {
				break
			} else {
				log.Fatal(err)
			}
		}

		regName, _ := seccomp.ScmpSyscall(regs.Orig_rax).GetName() // no-lib alternative would be to create a arch-dependent map[code]name

		if stage == "enter" {
			stage = "exit"
		} else {
			stage = "enter"
			syscallCounter[regName] += 1
		}

		// continue to the next syscall enter or exit
		syscall.PtraceSyscall(pid, 0)

		syscall.Wait4(pid, nil, 0, nil)

	}

	fmt.Println(">>>done")
	for k, v := range syscallCounter {
		fmt.Printf("%s -> %v \n", k, v)
	}
}
