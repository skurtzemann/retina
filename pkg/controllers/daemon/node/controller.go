// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package node

import (
	"context"
	"net"

	"github.com/microsoft/retina/pkg/common/apiretry"
	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	retinaCommon "github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/controllers/cache"
)

// NodeReconciler reconciles a Node object
type NodeReconciler struct {
	client.Client

	cache *cache.Cache
	l     *log.ZapLogger
}

func New(client client.Client, cache *cache.Cache) *NodeReconciler {
	return &NodeReconciler{
		Client: client,
		cache:  cache,
		l:      log.Logger().Named(string("NodeReconciler")),
	}
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list
func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// startTime := time.Now()
	r.l.Debug("Reconciling Node", zap.String("Node", req.NamespacedName.String()))

	/*
		// commenting out the latency calculation for now
		// we will need to create these as metrics
		defer func() {
			latency := time.Since(startTime).String()
			r.l.Info("Reconciliation ends", zap.String("Node", req.NamespacedName.String()), zap.String("latency", latency))
		}()
	*/
	node := &corev1.Node{}
	if err := apiretry.Do(
		func() error {
			return r.Client.Get(ctx, req.NamespacedName, node)
		},
	); err != nil {
		if errors.IsNotFound(err) {
			// Node deleted since reconcile request received.
			r.l.Debug("Node is not found", zap.String("Node", req.NamespacedName.String()))
			// It's safe to use req.NamespacedName as the Node and RetinaNode share the same namespace and name.
			if err := r.cache.DeleteRetinaNode(req.NamespacedName.String()); err != nil {
				r.l.Warn("Failed to delete RetinaNode in Cache from Node", zap.Error(err), zap.String("RetinaNode", req.NamespacedName.String()))
			}
			return ctrl.Result{}, nil
		}
		r.l.Error("Failed to fetch Node", zap.Error(err), zap.String("Node", req.NamespacedName.String()))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !node.ObjectMeta.DeletionTimestamp.IsZero() {
		r.l.Info("Node is being deleted", zap.String("Node", req.Name))
		if err := r.cache.DeleteRetinaNode(req.NamespacedName.String()); err != nil {
			r.l.Warn("Failed to delete RetinaNode in Cache from Node", zap.Error(err), zap.String("Node", req.NamespacedName.String()))
		}
		return ctrl.Result{}, nil
	}

	if len(node.Status.Addresses) == 0 {
		r.l.Warn("Node has no addresses", zap.String("Node", req.NamespacedName.String()))
		return ctrl.Result{}, nil
	}

	zone := r.extractZoneWithLog(node.Labels, node.Name)
	retinaNodeCommon := retinaCommon.NewRetinaNode(
		node.Name,
		net.ParseIP(node.Status.Addresses[0].Address),
		zone,
	)
	if err := r.cache.UpdateRetinaNode(retinaNodeCommon); err != nil {
		r.l.Error("Failed to update RetinaNode in Cache", zap.Error(err), zap.String("Node", req.NamespacedName.String()))
		return ctrl.Result{}, err
	}
	r.l.Debug("Updated RetinaNode in Cache",
		zap.String("node", node.Name),
		zap.String("zone", zone))

	return ctrl.Result{}, nil
}

func getLabelKeys(labels map[string]string) []string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	return keys
}

func (r *NodeReconciler) extractZoneWithLog(labels map[string]string, node string) string {
	if labels == nil {
		r.l.Info("Node has no labels, using fallback zone",
			zap.String("node", node),
			zap.String("reason", "labels_is_nil"))
		return retinaCommon.TopologyZoneLabelFallback
	}
	if az, ok := labels[retinaCommon.TopologyZoneLabelGA]; ok {
		r.l.Info("Found GA zone label on node",
			zap.String("node", node),
			zap.String("label", retinaCommon.TopologyZoneLabelGA),
			zap.String("zone", az))
		return az
	}
	if az, ok := labels[retinaCommon.TopologyZoneLabelBeta]; ok {
		r.l.Info("Found beta zone label on node (deprecated)",
			zap.String("node", node),
			zap.String("label", retinaCommon.TopologyZoneLabelBeta),
			zap.String("zone", az))
		return az
	}
	r.l.Info("No zone labels found on node",
		zap.String("node", node),
		zap.Strings("available_labels", getLabelKeys(labels)))
	return retinaCommon.TopologyZoneLabelFallback
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.l.Info("Setting up Node controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Complete(r)
}
