// Copyright 2020 Hewlett Packard Enterprise Development LP

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package node

import (
	"context"
	"fmt"
	"time"

	"github.com/bluek8s/kubedirector/pkg/shared"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_node")

// Add creates a new node Controller and adds it to the Manager.
// The Manager will set fields on the Controller and Start it when the Manager
// is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler.
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileNode{scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler.
func add(
	mgr manager.Manager,
	r reconcile.Reconciler,
) error {

	// Create a new controller
	c, err := controller.New("node-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource node.
	err = c.Watch(&source.Kind{Type: &corev1.Node{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileNode implements
// reconcile.Reconciler.
var _ reconcile.Reconciler = &ReconcileNode{}

const (
	// Period between the time when the controller requeues a request and
	// when it's scheduled again for reconciliation.
	reconcilePeriod = 30 * time.Second
)

// ReconcileNode reconciles a Node object.
type ReconcileNode struct {
	scheme *runtime.Scheme
}

// Reconcile watches the resource of type corev1.Node{} and
// acts if the node resource with a specific label
// has been modified.
func (r *ReconcileNode) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reconcileResult := reconcile.Result{RequeueAfter: reconcilePeriod}

	// Fetch the Node instance.
	node := &corev1.Node{}

	err := shared.Get(context.TODO(), request.NamespacedName, node)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcileResult,
			fmt.Errorf("could not fetch Node instance: %s", err)
	}

	err = r.syncNode(reqLogger, node)
	return reconcileResult, err
}
