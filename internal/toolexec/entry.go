package toolexec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ListenOcean/goHookTool/configs"
	"github.com/ListenOcean/goHookTool/internal/toolexec/flags"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var globalFlags flags.InstrumentationToolFlagSet

var ToolexecCmd = &cobra.Command{
	Use:                "toolexec",
	Short:              "Provided for -toolexec, not user.",
	Run:                ToolexecEntry,
	DisableFlagParsing: true,
}

func CheckAndCleanLogFile(filename string) {
	checkInterval := time.Hour * 24 * 3
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return
	}

	if time.Since(fileInfo.ModTime()) > checkInterval {
		err = os.Truncate(filename, 0)
		if err != nil {
			return
		}
	}
}

func ToolexecEntry(cobracmd *cobra.Command, args []string) {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile | log.Lmsgprefix)
	log.SetPrefix(time.Now().Format(time.RFC3339Nano) + ":\t")

	logFileName := os.TempDir() + "/.autobuild_go_toolexec.log"
	CheckAndCleanLogFile(logFileName)
	logFile, err := os.OpenFile(logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.SetOutput(os.Stdout)
		log.Printf("open logfile failed: %s", err.Error())
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.Printf("\n\nToolexec Start\n")

	cmd, cmdArgPos, err := parseCommand(&globalFlags, args)
	if err != nil || globalFlags.Help {
		log.Println(err)
		printUsage()
		os.Exit(1)
	}

	// Hide instrumentation tool arguments
	if cmdArgPos != -1 {
		args = args[cmdArgPos:]
	}

	// var logs strings.Builder
	if !globalFlags.Verbose {
		// Save the logs to show them in case of instrumentation error
		log.SetOutput(logFile)
	}

	// 读取Hook点配置
	if err := ReadConfig(); err != nil {
		log.Println("Found no config, maybe no hookpoints")
	}

	log.Printf("origin command \"%s\"", strings.Join(args, "\", \""))
	if cmd != nil {
		// The command is implemented
		newArgs, err := cmd()
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
		if newArgs != nil {
			// Args are replaced
			args = newArgs
		}
	}

	err = forwardCommand(args)
	var exitErr *exec.ExitError
	if err != nil {
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		} else {
			log.Fatalln(err)
		}
	}
	log.Printf("\nToolexec End\n\n")
	os.Exit(0)
}

func ReadConfig() error {
	configFile := os.Getenv(configs.TagCustomConfig)
	if configFile == "" {
		return errors.New("no config file")
	}
	datas, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(datas, &configs.ConfigData)
	if err != nil {
		return err
	}
	// convert to HookPointMap
	for key, values := range configs.ConfigData.Hookpoints {
		configs.HookPointMap[key] = make(map[string]struct{})
		for _, value := range values {
			configs.HookPointMap[key][value] = struct{}{}
		}
	}
	return nil
}

// forwardCommand runs the given command's argument list and exits the process
// with the exit code that was returned.
func forwardCommand(args []string) error {
	var stdout, stderr bytes.Buffer
	path := args[0]
	args = args[1:]
	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	quotedArgs := fmt.Sprintf("%+q", args)
	log.Printf("forwarding command `%s %s`", path, quotedArgs[1:len(quotedArgs)-1])
	err := cmd.Run()
	stdoutStr := stdout.String()
	stderrStr := stderr.String()
	io.Copy(os.Stdout, &stdout)
	io.Copy(os.Stderr, &stderr)
	log.Printf("stdout: %s", stdoutStr)
	log.Printf("stderr: %s", stderrStr)
	return err
}

func printUsage() {
	const usageFormat = `Nothing ToDo`
	_, _ = fmt.Fprintf(os.Stderr, usageFormat)
	os.Exit(2)
}

type parseCommandFunc func([]string) (commandExecutionFunc, error)
type commandExecutionFunc func() (newArgs []string, err error)

var commandParserMap = map[string]parseCommandFunc{
	"compile": parseCompileCommand,
}

// getCommand returns the command and arguments. The command is expectedFlags to be
// the first argument.
func parseCommand(instrToolFlagSet *flags.InstrumentationToolFlagSet, args []string) (commandExecutionFunc, int, error) {
	cmdIdPos := flags.ParseFlagsUntilFirstNonOptionArg(instrToolFlagSet, args)
	if cmdIdPos == -1 {
		return nil, cmdIdPos, errors.New("unexpected arguments")
	}
	cmdId := args[cmdIdPos]
	args = args[cmdIdPos:]
	cmdId, err := parseCommandID(cmdId)
	if err != nil {
		return nil, cmdIdPos, err
	}

	if commandParser, exists := commandParserMap[cmdId]; exists {
		cmd, err := commandParser(args)
		return cmd, cmdIdPos, err
	} else {
		return nil, cmdIdPos, nil
	}
}

// parseCommandID 解析调用的工具（compile等）
func parseCommandID(cmd string) (string, error) {
	// It mustn't be empty
	if cmd == "" {
		return "", errors.New("unexpected empty command name")
	}

	// Take the base of the absolute path of the go tool
	cmd = filepath.Base(cmd)
	// Remove the file extension if any
	if ext := filepath.Ext(cmd); ext != "" {
		cmd = strings.TrimSuffix(cmd, ext)
	}
	return cmd, nil
}
