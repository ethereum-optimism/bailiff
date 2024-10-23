package bailiff

import (
	"bufio"
	"context"
	_ "embed"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/ethereum/go-ethereum/log"
)

//go:embed repush.sh
var scriptSrc string

type ShellRepusher struct {
	lgr            log.Logger
	workdir        string
	privateKeyFile string
	mtx            sync.Mutex
}

type Repusher interface {
	Repush(ctx context.Context, forkRepo, srcBranch, upstreamBranch, requestedSHA string) error
}

func NewShellRepusher(lgr log.Logger, workdir string, privateKeyFile string) *ShellRepusher {
	return &ShellRepusher{
		lgr:            lgr,
		workdir:        workdir,
		privateKeyFile: privateKeyFile,
	}
}

func (s *ShellRepusher) Clone(ctx context.Context, repoURL string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	env := fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new", s.privateKeyFile)
	cmd := exec.CommandContext(
		ctx,
		"git",
		"clone",
		repoURL,
		".",
	)
	cmd.Dir = s.workdir
	cmd.Env = append(cmd.Env, env)

	doneC := make(chan struct{})
	if err := s.logOutput(cmd, doneC); err != nil {
		return fmt.Errorf("output logger setup failed: %s", err)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command execution failed: %s", err)
	}

	<-doneC

	return nil
}

func (s *ShellRepusher) Repush(ctx context.Context, forkRepo, srcBranch, upstreamBranch, requestedSHA string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	cmd := exec.CommandContext(
		ctx,
		"bash",
		"-c",
		scriptSrc,
		forkRepo,
		srcBranch,
		upstreamBranch,
		requestedSHA,
		s.privateKeyFile,
	)
	cmd.Dir = s.workdir

	doneC := make(chan struct{})
	if err := s.logOutput(cmd, doneC); err != nil {
		return fmt.Errorf("output logger setup failed: %s", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("command start failed: %s", err)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("command execution failed: %s", err)
	}

	<-doneC

	return nil
}

func (s *ShellRepusher) logOutput(cmd *exec.Cmd, doneC chan struct{}) error {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe creation failed: %s", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe creation failed: %s", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	outputLogger := func(r io.Reader, prefix string) {
		defer wg.Done()
		scan := bufio.NewScanner(r)
		for scan.Scan() {
			s.lgr.Info(fmt.Sprintf("[%s]: %s", prefix, scan.Text()))
		}
	}

	go outputLogger(stdoutPipe, "stdout")
	go outputLogger(stderrPipe, "stderr")
	go func() {
		wg.Wait()
		close(doneC)
	}()

	return nil
}
