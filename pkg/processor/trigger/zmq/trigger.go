/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package zmq

import (
	"time"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	zmq4 "github.com/pebbe/zmq4"
)

type zeroMq struct {
	trigger.AbstractTrigger
	event         Event
	configuration *Configuration
	context       *zmq4.Context
	socket        *zmq4.Socket
	worker        *worker.Worker
}

func newTrigger(parentLogger logger.Logger, workerAllocator worker.Allocator, configuration *Configuration) (trigger.Trigger, error) {

	newTrigger := zeroMq{
		AbstractTrigger: trigger.AbstractTrigger{
			ID:              configuration.ID,
			Logger:          parentLogger.GetChild(configuration.ID),
			WorkerAllocator: workerAllocator,
			Class:           "async",
			Kind:            "zeroMq",
		},
		configuration: configuration,
	}

	return &newTrigger, nil
}

func (zmq *zeroMq) Start(checkpoint functionconfig.Checkpoint) error {
	var err error

	zmq.Logger.InfoWith("Starting", "socketURL", zmq.configuration.SocketURL)

	// get a worker, we'll be using this one always
	zmq.worker, err = zmq.WorkerAllocator.Allocate(10 * time.Second)
	if err != nil {
		return errors.Wrap(err, "Failed to allocate worker")
	}

	// zmq.setEmptyParameters()
	zmq.context, _ = zmq4.NewContext()
	zmq.socket, _ = zmq.context.NewSocket(zmq4.REQ)
	zmq.socket.SetSubscribe(zmq.configuration.Subscribe)
	// if err := zmq.createBrokerResources(); err != nil {
	// 	return errors.Wrap(err, "Failed to create broker resources")
	// }

	// start listening for published messages
	go zmq.handleBrokerMessages()

	return nil
}

func (zmq *zeroMq) Stop(force bool) (functionconfig.Checkpoint, error) {

	// TODO
	return nil, nil
}

func (zmq *zeroMq) GetConfig() map[string]interface{} {
	return common.StructureToMap(zmq.configuration)
}

func (zmq *zeroMq) handleBrokerMessages() {
	defer zmq.socket.Close()
	for {
		message, _ := zmq.socket.RecvMessage(0)

		for _, item := range message {
			// bind to delivery
			zmq.event.message = item

			// submit to worker
			_, submitError, _ := zmq.AllocateWorkerAndSubmitEvent(&zmq.event, nil, 10*time.Second)

			// ack the message if we didn't fail to submit
			if submitError == nil {
				// message.Ack(false) // nolint: errcheck
			} else {
				zmq.Logger.WarnWith("Failed to submit to worker", "err", submitError)
			}
		}
	}
}
