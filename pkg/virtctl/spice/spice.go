package spice

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"kubevirt.io/client-go/kubecli"

	"kubevirt.io/kubevirt/pkg/virtctl/templates"
	"kubevirt.io/kubevirt/pkg/virtctl/vnc/screenshot"
)

const FLAG = "spice"
const TEMP_PREFIX = "spice"

var server = "127.0.0.1"
var kubeconfig = ""

var details bool

func NewCommand(clientConfig clientcmd.ClientConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "spice (VMI)",
		Short:   "Open a spice connection to a virtual machine instance.",
		Example: usage(),
		Args:    templates.ExactArgs("spice", 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := Spice{clientConfig: clientConfig}
			return c.Run(cmd, args)
		},
	}
	//cmd.Flags().StringVar(&server, "server", server, "--address=127.0.0.1")
	//cmd.Flags().StringVar(&kubeconfig, "kubeconfig", kubeconfig, "--address=127.0.0.1")
	//cmd.Flags().BoolVar(&details, "d", details, "If present, print SPICE console to stdout, otherwise run remote-viewer")
	//cmd.Flags().IntVar(&customPort, "port", customPort,
	//	"--port=0: Assigning a port value to this will try to run the proxy on the given port if the port is accessible; If unassigned, the proxy will run on a random port")
	cmd.SetUsageTemplate(templates.UsageTemplate())
	cmd.AddCommand(screenshot.NewScreenshotCommand(clientConfig))
	return cmd
}

func usage() string {
	return `  # Connect to 'testvmi' via remote-viewer:
   {{ProgramName}} spice testvmi`
}

type Spice struct {
	clientConfig clientcmd.ClientConfig
}

func (o *Spice) Run(cmd *cobra.Command, args []string) error {
	namespace, _, err := o.clientConfig.Namespace()
	if err != nil {
		return err
	}

	vmi := args[0]

	virtCli, err := kubecli.GetKubevirtClientFromClientConfig(o.clientConfig)
	if err != nil {
		return err
	}

	// setup connection with VM
	spice, err := virtCli.VirtualMachineInstance(namespace).Spice(vmi)

	if err != nil {
		return fmt.Errorf("Can't access VMI %s: %s", vmi, err.Error())
	}

	lnAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("Can't resolve the address: %s", err.Error())
	}

	ln, err := net.ListenTCP("tcp", lnAddr)
	if err != nil {
		return fmt.Errorf("Can't listen on unix socket: %s", err.Error())
	}

	pipeInReader, pipeInWriter := io.Pipe()
	pipeOutReader, pipeOutWriter := io.Pipe()

	k8ResChan := make(chan error)
	listenResChan := make(chan error)
	viewResChan := make(chan error)
	stopChan := make(chan struct{}, 1)
	doneChan := make(chan struct{}, 1)
	writeStop := make(chan error)
	readStop := make(chan error)

	go func() {

		k8ResChan <- spice.Stream(kubecli.StreamOptions{
			In:  pipeInReader,
			Out: pipeOutWriter,
		})
	}()

	go func() {

		fd, err := ln.Accept()
		if err != nil {
			glog.V(2).Infof("Failed to accept unix sock connection. %s", err.Error())
			listenResChan <- err
		}
		defer fd.Close()

		templates.PrintWarningForPausedVMI(virtCli, vmi, namespace)

		// write to FD <- pipeOutReader
		go func() {
			_, err := io.Copy(fd, pipeOutReader)
			readStop <- err
		}()

		// read from FD -> pipeInWriter
		go func() {
			_, err := io.Copy(pipeInWriter, fd)
			writeStop <- err
		}()

		// don't terminate until vnc client is done
		<-doneChan
		listenResChan <- err
	}()

	port := ln.Addr().(*net.TCPAddr).Port

	go checkAndRunVNCViewer(doneChan, port)

	go func() {
		defer close(stopChan)
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)
		<-interrupt
	}()

	select {
	case <-stopChan:
	case err = <-readStop:
	case err = <-writeStop:
	case err = <-k8ResChan:
	case err = <-viewResChan:
	case err = <-listenResChan:
	}

	if err != nil {
		return fmt.Errorf("Error encountered: %s", err.Error())
	}
	return nil
}

func checkAndRunVNCViewer(doneChan chan struct{}, port int) {
	defer close(doneChan)
	var err error
	args := []string{}

	args = remoteViewerArgs(port)
	cmnd := exec.Command("remote-viewer", args...)
	output, err := cmnd.CombinedOutput()
	if err != nil {
		glog.Errorf("execution failed: %v, output: %v", err, string(output))
	} else {
		glog.V(2).Infof("output: %v", string(output))
	}
}

func remoteViewerArgs(port int) (args []string) {
	args = append(args, fmt.Sprintf("spice://127.0.0.1:%d", port))
	if glog.V(4) {
		args = append(args, "--debug")
	}
	return
}
