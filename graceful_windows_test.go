// +build windows

package graceful

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

const sigint = int(128 + syscall.SIGINT)

func TestHelperSleep(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	d, err := time.ParseDuration(os.Getenv("SLEEP_DURATION"))
	if err != nil {
		return
	}
	time.Sleep(d)
}

/*
func TestHelperUndying(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	d, err := time.ParseDuration(os.Getenv("DEATH_DURATION"))
	if err != nil {
		return
	}
	defer time.Sleep(d)
	for {
		time.Sleep(time.Second)
	}
}
*/

func ExampleExit() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cmd := exec.Command("ping", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		fmt.Printf("unable to start ping: %v", err)
		os.Exit(2)
	}
	fmt.Println("ping started")
	if err := Exit(ctx, cmd.Process.Pid, -1); err != nil {
		fmt.Printf("exit failed: %v", err)
		cmd.Process.Kill() // Forcefully end the process (but know that this won't kill child processes)
		os.Exit(2)
	}
	cmd.Wait()
	fmt.Println("ping exited")
	// Output:
	// ping started
	// ping exited
}

func TestExit(t *testing.T) {
	cmd := execSleep(t, time.Second)
	Exit(context.Background(), cmd.Process.Pid, sigint)
	cmd.Wait()
	if err := checkExitCode(cmd, sigint); err != nil {
		t.Fatal(err)
	}
}

func TestExitCancellation(t *testing.T) {
	cmd := execSleep(t, time.Millisecond*200)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	Exit(ctx, cmd.Process.Pid, sigint)
	err := Exit(ctx, cmd.Process.Pid, sigint)
	if err != context.Canceled {
		t.Fatalf("unexpected exit error \"%v\" when \"%s\" was expected", err, context.Canceled)
	}
	cmd.Wait()
	if err := checkExitCode(cmd, 0); err != nil {
		t.Fatal(err)
	}
}

func TestExitDeadline(t *testing.T) {
	cmd := execSleep(t, time.Millisecond*200)
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	err := Exit(ctx, cmd.Process.Pid, sigint)
	if err != context.DeadlineExceeded {
		t.Fatalf("unexpected shutdown error \"%v\" when \"%s\" was expected", err, context.DeadlineExceeded)
	}
	cmd.Wait()
	if err := checkExitCode(cmd, 0); err != nil {
		t.Fatal(err)
	}
}

func TestExitBadPID(t *testing.T) {
	err := Exit(context.Background(), 4322341234, sigint)
	if err == nil || err.Error() != "OpenProcess: The parameter is incorrect." {
		t.Fatalf("unexpected exit error \"%v\" when \"%s\" was expected", err, syscall.EINVAL)
	}
}

func TestTerminate(t *testing.T) {
	cmd := execSleep(t, time.Second)
	Terminate(cmd.Process.Pid, sigint)
	cmd.Wait()
	if err := checkExitCode(cmd, 1); err != nil {
		t.Fatal(err)
	}
}

func execSleep(t *testing.T, d time.Duration) (cmd *exec.Cmd) {
	cmd = exec.Command(os.Args[0], "-test.run=TestHelperSleep")
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1", fmt.Sprintf("SLEEP_DURATION=%s", d)}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return
}

/*
func execUndying(t *testing.T, d time.Duration) (cmd *exec.Cmd) {
	cmd = exec.Command(os.Args[0], "-test.run=TestHelperUndying")
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1", fmt.Sprintf("DEATH_DURATION=%s", d)}
	//cmd.Stdout = os.Stdout
	//cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return
}
*/

func checkExitCode(cmd *exec.Cmd, expected int) (err error) {
	exitCode, err := exitCode(cmd)
	if err != nil {
		return err
	}
	if exitCode != expected {
		return fmt.Errorf("command returned exit code %d instead of expected exit code %d", exitCode, expected)
	}
	return nil
}

func exitCode(cmd *exec.Cmd) (int, error) {
	status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ok {
		return 0, errors.New("unable to analyze process state due to unexpected sys type")
	}
	return int(status.ExitCode), nil
}
