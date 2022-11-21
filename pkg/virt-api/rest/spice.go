package rest

import (
	"fmt"

	restful "github.com/emicklei/go-restful"
	"k8s.io/apimachinery/pkg/api/errors"

	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/client-go/log"
)

func (app *SubresourceAPIApp) SpiceRequestHandler(request *restful.Request, response *restful.Response) {
	//activeConnectionMetric := apimetrics.NewActiveVNCConnection(request.PathParameter("namespace"), request.PathParameter("name"))
	//defer activeConnectionMetric.Dec()
	streamer := NewRawStreamer(
		app.FetchVirtualMachineInstance,
		validateVMIForSpice,
		app.virtHandlerDialer(func(vmi *v1.VirtualMachineInstance, conn kubecli.VirtHandlerConn) (string, error) {
			return conn.SpiceURI(vmi)
		}),
	)
	log.Log.Infof("SpiceRequestHandler")
	streamer.Handle(request, response)
}

func validateVMIForSpice(vmi *v1.VirtualMachineInstance) *errors.StatusError {
	// If there are no graphics devices present, we can't proceed
	if vmi.Spec.Domain.Devices.AutoattachGraphicsDevice != nil && *vmi.Spec.Domain.Devices.AutoattachGraphicsDevice == false {
		err := fmt.Errorf("No graphics devices are present.")
		log.Log.Object(vmi).Reason(err).Error("Can't establish Spice connection.")
		return errors.NewBadRequest(err.Error())
	}
	return nil
}
