package rest

import (
	"context"
	"crypto/md5"
	"encoding/json"
	goerror "errors"
	"fmt"
	"github.com/emicklei/go-restful"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/client-go/log"
	"net/http"
	"strings"
)

func generateToken() string {
	return fmt.Sprintf("%x", md5.Sum([]byte(rand.String(48))))
}

func (app *SubresourceAPIApp) SpiceRequestHandler(request *restful.Request, response *restful.Response) {
	name := request.PathParameter("name")
	namespace := request.PathParameter("namespace")

	vmi, err := app.FetchVirtualMachineInstance(name, namespace)
	if err != nil {
		response.WriteError(http.StatusBadRequest,err)
		return
	}

	if !vmi.IsRunning() {
		response.WriteError(http.StatusBadRequest, goerror.New(fmt.Sprintf("Unable to connect to VirtualMachineInstance because phase is %s instead of %s", vmi.Status.Phase, v1.Running)))
		return
	}

	// should not occur if the vm is handled by the virt-handler and in running state
	if vmi.Status.SpiceConnection == nil {
		response.WriteError(http.StatusInternalServerError, goerror.New(fmt.Sprintf("spice connection is not configured")))
		return
	}

	token := generateToken()
	spiceToken := &v1.SpiceToken{ExparationTime: k8smetav1.Now(), Token: token}
	spiceTokenStr, _ := json.Marshal(spiceToken)

	updateLabel := fmt.Sprintf(`{"op": "replace", "path": "/metadata/labels/%s", "value": "%s" }`, v1.SpiceTokenLabel, token)
	updateToken := fmt.Sprintf(`{"op": "replace", "path": "/status/spiceConnection/spiceToken", "value": %s }`, string(spiceTokenStr))

	data := fmt.Sprintf("[ %s , %s ]", updateLabel, updateToken)
	patchType := types.JSONPatchType
	_, err = app.virtCli.VirtualMachineInstance(namespace).Patch(vmi.GetName(), patchType, []byte(data))
	if err != nil {
		errCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "jsonpatch test operation does not apply") {
			errCode = http.StatusConflict
		}
		response.WriteError(errCode, fmt.Errorf("%v: %s", err, data))
		return
	}

	svc, err := app.virtCli.CoreV1().Services(app.KubevirtNamespace).Get(context.Background(),v1.SpiceServiceName,
		k8smetav1.GetOptions{})
	if err != nil {
		response.WriteError(http.StatusInternalServerError, goerror.New(fmt.Sprintf("failed to find spice service error: %v", err)))
		return
	}

	if len(svc.Status.LoadBalancer.Ingress) == 0 {
		log.Log.Info("no loadbalancer ip address found for spice service")
		response.WriteError(http.StatusInternalServerError, goerror.New("no loadbalancer ip address found for spice service"))
		return
	}

	spiceOptions := kubecli.SpiceOptions{Host: svc.Status.LoadBalancer.Ingress[0].IP,
		Port:  svc.Spec.Ports[0].Port,
		Token: token}

	response.WriteAsJson(spiceOptions)
	response.WriteHeader(http.StatusAccepted)
}

