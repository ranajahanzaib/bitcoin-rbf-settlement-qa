package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var ErrEnvironment = errors.New("unsupported environment")

type Node struct {
	RPC     *RPC
	Wallet  *RPC
	Dir     string
	Port    int
	Version int
	Policy  map[string]any
	cmd     *exec.Cmd
	done    chan error
	log     *os.File
}

func port() (int, error) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return 0, e
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
func Start(ctx context.Context, binary string) (n *Node, err error) {
	if _, e := exec.LookPath(binary); e != nil {
		return nil, fmt.Errorf("%w: install Core 32.1: %v", ErrEnvironment, e)
	}
	d, e := os.MkdirTemp("", "rbfqa-node-")
	if e != nil {
		return nil, e
	}
	n = &Node{Dir: d, done: make(chan error, 1)}
	owned := n
	defer func() {
		if err != nil {
			data, _ := os.ReadFile(filepath.Join(owned.Dir, "process.log"))
			if len(data) > 4096 {
				data = data[len(data)-4096:]
			}
			if len(data) > 0 {
				detail := string(data)
				if owned.RPC != nil {
					detail = strings.ReplaceAll(detail, owned.RPC.Password, "[redacted]")
				}
				err = fmt.Errorf("%w; node startup log: %s", err, strings.TrimSpace(detail))
			}
			owned.Close(false)
		}
	}()
	rp, e := port()
	if e != nil {
		return nil, e
	}
	n.Port, e = port()
	if e != nil {
		return nil, e
	}
	secret := make([]byte, 24)
	if _, e = rand.Read(secret); e != nil {
		return nil, e
	}
	pass := hex.EncodeToString(secret)
	n.RPC = &RPC{URL: fmt.Sprintf("http://127.0.0.1:%d", rp), User: "rbfqa", Password: pass}
	n.Wallet = &RPC{URL: n.RPC.URL + "/wallet/sender", User: "rbfqa", Password: pass}
	n.log, e = os.Create(filepath.Join(d, "process.log"))
	if e != nil {
		return nil, e
	}
	n.cmd = exec.Command(binary, "-regtest", "-server", "-listen=1", "-bind=127.0.0.1", "-discover=0", "-listenonion=0", "-natpmp=0", "-dnsseed=0", "-fixedseeds=0", "-txindex=1", "-fallbackfee=0.0002", "-datadir="+d, "-rpcbind=127.0.0.1", "-rpcallowip=127.0.0.1", "-rpcuser=rbfqa", "-rpcpassword="+pass, "-rpcport="+strconv.Itoa(rp), "-port="+strconv.Itoa(n.Port))
	n.cmd.Stdout = n.log
	n.cmd.Stderr = n.log
	if e = n.cmd.Start(); e != nil {
		return nil, e
	}
	cmd, done := n.cmd, n.done
	go func() { done <- cmd.Wait() }()
	ready, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	e = Wait(ready, func() (bool, error) {
		var b struct {
			Chain string `json:"chain"`
		}
		e := n.RPC.Call(ready, "getblockchaininfo", &b)
		if e == nil && b.Chain != "regtest" {
			return false, fmt.Errorf("unsafe network %s", b.Chain)
		}
		return b.Chain == "regtest", e
	})
	if e != nil {
		return nil, e
	}
	var network struct {
		Version int `json:"version"`
	}
	if e = n.RPC.Call(ctx, "getnetworkinfo", &network); e != nil {
		return nil, e
	}
	n.Version = network.Version
	if n.Version != 320100 {
		return nil, fmt.Errorf("%w: Core version %d: require 32.1 (320100)", ErrEnvironment, n.Version)
	}
	n.Policy = map[string]any{}
	if e = n.RPC.Call(ctx, "getmempoolinfo", &n.Policy); e != nil {
		return nil, e
	}
	for _, method := range []string{"createrawtransaction", "signrawtransactionwithwallet", "testmempoolaccept", "getblock", "setnetworkactive"} {
		var h string
		if e = n.RPC.Call(ctx, "help", &h, method); e != nil {
			return nil, fmt.Errorf("required capability %s: %w", method, e)
		}
	}
	if e = n.RPC.Call(ctx, "createwallet", nil, "sender"); e != nil {
		return nil, e
	}
	return n, nil
}
func (n *Node) Close(keep bool) {
	if n == nil {
		return
	}
	if n.cmd != nil && n.cmd.Process != nil {
		ctx, c := context.WithTimeout(context.Background(), 3*time.Second)
		_ = n.RPC.Call(ctx, "stop", nil)
		c()
		select {
		case <-n.done:
		case <-time.After(5 * time.Second):
			_ = n.cmd.Process.Kill()
			<-n.done
		}
	}
	if n.log != nil {
		_ = n.log.Close()
	}
	if !keep {
		_ = os.RemoveAll(n.Dir)
	}
}
func (n *Node) Address(ctx context.Context) (string, error) {
	var a string
	e := n.Wallet.Call(ctx, "getnewaddress", &a, "", "bech32")
	return a, e
}
func (n *Node) Mine(ctx context.Context, count int) error {
	a, e := n.Address(ctx)
	if e != nil {
		return e
	}
	for count > 0 {
		chunk := count
		if chunk > 10 {
			chunk = 10
		}
		if err := n.RPC.Call(ctx, "generatetoaddress", nil, chunk, a); err != nil {
			return err
		}
		count -= chunk
	}
	return nil
}
func (n *Node) Connect(ctx context.Context, other *Node) error {
	return n.RPC.Call(ctx, "addnode", nil, fmt.Sprintf("127.0.0.1:%d", other.Port), "onetry")
}
func Sync(ctx context.Context, a, b *Node) error {
	return Wait(ctx, func() (bool, error) {
		var x, y string
		if e := a.RPC.Call(ctx, "getbestblockhash", &x); e != nil {
			return false, e
		}
		e := b.RPC.Call(ctx, "getbestblockhash", &y)
		return x == y, e
	})
}
