package l1starter

import (
	"bufio"
	"context"
	_ "embed"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/samber/lo"

	"github.com/iotaledger/iota.go/v4/nodeclient"
	"github.com/iotaledger/wasp/packages/l1connection"
	"github.com/iotaledger/wasp/packages/util"
)

var (
	//go:embed .env
	configENV string
	//go:embed config.json
	configJSON string
	//go:embed docker-compose.yml
	configDockerCompose string
	//go:embed docker-network.snapshot
	configSnapshot string
)

// requires `docker` and `docker compose` installed
type L1Starter struct {
	Config     l1connection.Config
	started    bool
	workingDir string
}

var defaultConfig = l1connection.Config{
	APIAddress:    "http://0.0.0.0:8050", // TODO only related to node #0
	INXAddress:    "http://0.0.0.0:9059", // TODO only related to node #0
	FaucetAddress: "http://0.0.0.0:8088",
}

// New sets up the CLI flags relevant to L1/privtangle configuration in the given FlagSet.
func New(l1flags, inxFlags *flag.FlagSet) *L1Starter {
	s := &L1Starter{}
	l1flags.StringVar(&s.Config.APIAddress, "layer1-api", "", "layer1 API address")
	inxFlags.StringVar(&s.Config.INXAddress, "layer1-inx", "", "layer1 INX address")
	l1flags.StringVar(&s.Config.FaucetAddress, "layer1-faucet", "", "layer1 faucet port")
	return s
}

func (s *L1Starter) PrivtangleEnabled() bool {
	return s.Config.APIAddress == ""
}

type LogFunc func(format string, args ...any)

func (s *L1Starter) runCmd(cmd *exec.Cmd, log LogFunc) {
	util.TerminateCmdWhenTestStops(cmd)
	cmd.Dir = s.workingDir
	// combine output of stdout and stderr and print using the provided log func
	stdOut := lo.Must(cmd.StdoutPipe())
	stdErr := lo.Must(cmd.StderrPipe())
	r := io.MultiReader(stdOut, stdErr)
	lo.Must0(cmd.Start())
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		log(scanner.Text())
	}
	err := cmd.Wait()
	if err != nil {
		log(err.Error()) // log error, do not panic, otherwise the output of the command will not be logged
	}
}

func (s *L1Starter) setupWorkingDir() {
	dir, err := os.MkdirTemp(os.TempDir(), "privtangle-*")
	if err != nil {
		panic(err)
	}
	s.workingDir = dir
	writefile := func(filename, content string) {
		err := os.WriteFile(filepath.Join(s.workingDir, filename), []byte(content), 0o644) //nolint:gosec // we need these permissions
		if err != nil {
			panic(err)
		}
	}
	writefile(".env", configENV)
	writefile("config.json", configJSON)
	writefile("docker-compose.yml", configDockerCompose)
	writefile("docker-network.snapshot", configSnapshot)
}

// StartPrivtangleIfNecessary starts a private tangle, unless an L1 host was provided via cli flags
func (s *L1Starter) StartPrivtangleIfNecessary(log LogFunc) {
	if s.Config.APIAddress != "" || s.started {
		return
	}
	s.started = true
	s.Config = defaultConfig

	s.setupWorkingDir()

	s.Cleanup(log) // cleanup, just in case something is lingering from a previous execution
	// TODO  image `docker-network-node-1-validator` is built using iota-core repo. will they release an image?
	// anyway, for now to build the image, just pull iota-core repo, then `cd tools/docker-network && bash run.sh`, then kill the process and you should have the image ready

	// start the l1 network using the pre-built snapshot
	go s.runCmd(exec.Command("docker", "compose", "up"), log)

	s.WaitReady(log)
}

func (s *L1Starter) WaitReady(log LogFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	select {
	case <-time.After(2 * time.Minute):
		panic("timeout waiting for privtangle to be ready")
	case <-s.nodesReady(ctx, log):
		return
	}
}

func (s *L1Starter) nodesReady(ctx context.Context, log LogFunc) <-chan bool {
	readyChan := make(chan bool)
	go func() {
		var client *nodeclient.Client
		var err error
		for {
			// wait to be ready to create a client
			client, err = nodeclient.New(s.Config.APIAddress)
			if err == nil {
				break
			}
			time.Sleep(10 * time.Second)
		}
		for {
			// wait for the node to be synced
			resp, err := client.BlockIssuance(ctx)
			if err != nil {
				log("nodes not healthy, retrying. %s", err.Error())
			}
			if err == nil && resp != nil {
				break
			}
			time.Sleep(10 * time.Second)
		}
		for {
			// wait for indexer
			_, err := client.Indexer(ctx)
			if err == nil {
				readyChan <- true
				return
			}
			time.Sleep(10 * time.Second)
		}
	}()
	return readyChan
}

func (s *L1Starter) Pause(log LogFunc) {
	s.runCmd(exec.Command("docker", "compose", "pause"), log)
}

func (s *L1Starter) Resume(log LogFunc) {
	s.runCmd(exec.Command("docker", "compose", "unpause"), log)
}

func (s *L1Starter) Cleanup(log LogFunc) {
	s.runCmd(exec.Command("docker", "compose", "down", "-v"), log)
}
