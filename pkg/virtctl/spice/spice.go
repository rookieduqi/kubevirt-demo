package spice


import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"

	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/kubevirt/pkg/virtctl/templates"
)

func NewCommand(clientConfig clientcmd.ClientConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "spice (VMI)",
		Short:   "Open a spice connection to a virtual machine instance.",
		Example: usage(),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := Spice{clientConfig: clientConfig}
			return c.Run(cmd, args)
		},
	}
	cmd.SetUsageTemplate(templates.UsageTemplate())
	return cmd
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

	// get connection token for VM
	spiceOption, err := virtCli.VirtualMachineInstance(namespace).Spice(vmi)
	if err != nil {
		return fmt.Errorf("Can't access VMI %s: %s", vmi, err.Error())
	}

	dir, err := os.Getwd()
	spiceConfig := fmt.Sprintf(`[virt-viewer]
type=spice
host=%s
port=%d
password=%s
`, spiceOption.Host, spiceOption.Port, spiceOption.Token)

	return ioutil.WriteFile(path.Join(dir, fmt.Sprintf("%s.%s", vmi, "vv")), []byte(spiceConfig), 0644)
}

func usage() string {
	usage := "  # Connect to 'testvmi' via remote-viewer:\n"
	usage += "  virtctl spice testvmi"
	return usage
}

