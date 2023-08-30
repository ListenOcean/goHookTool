package build

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ListenOcean/goHookTool/internal/build/log"
)

func ForwardBuild() error {
	args, err := AddToolexec()
	if err != nil {
		return err
	}
	if err = DoBuildWithToolexec(args); err != nil {
		return err
	}
	return nil
}

func DoBuildWithToolexec(args []string) (err error) {
	if len(os.Args) < 2 {
		return fmt.Errorf("not enough args")
	}
	if _, _, err = ExecuteCmd(WorkDir, GoPath, args[1:], nil); err != nil {
		return
	}
	return nil
}

func IsToolexecExist(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-toolexec") {
			return true
		}
	}
	return false
}

func AddToolexec() (args []string, err error) {
	var toolexecStr string
	if !IsToolexecExist(os.Args) {
		toolexecStr = `-toolexec=` + AutobuildPath + ` toolexec`
		args = append(os.Args[:2], append([]string{toolexecStr, "-a"}, os.Args[2:]...)...)
	} else {
		log.Info("already has toolexec, skip hook")
		args = os.Args
	}
	return
}

func ExecuteCmd(workdir string, program string, args []string, env []string) (stdout, stderr []byte, err error) {
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd := exec.Command(program, args...)
	cmd.Dir = workdir
	cmd.Stderr = &stdoutBuf
	cmd.Stdout = &stderrBuf
	log.Debug(
		"Exec Command.",
		log.String("workdir", cmd.Dir),
		log.String("program", cmd.Path),
		log.String("args", strings.Join(cmd.Args, " ")),
	)
	cmd.Env = append(os.Environ(), env...)
	err = cmd.Run()
	stdout = stdoutBuf.Bytes()
	stderr = stderrBuf.Bytes()
	fmt.Println(stdoutBuf.String())
	fmt.Println(stderrBuf.String())
	if err != nil {
		log.Error(
			"Exec Result.",
			log.String("stdout", stdoutBuf.String()),
			log.String("stderr", stderrBuf.String()),
		)
		return
	}
	log.Debug(
		"Exec Result.",
		log.String("stdout", stdoutBuf.String()),
		log.String("stderr", stderrBuf.String()),
	)
	return
}
