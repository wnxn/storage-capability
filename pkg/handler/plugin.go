package handler

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"time"

	"context"
	librpc "github.com/kubernetes-csi/csi-lib-utils/rpc"
	"github.com/wnxn/storage-capability/pkg/apis/storagecapability/v1alpha1"
	"google.golang.org/grpc"
)

type PluginHandler interface {
	GetFullCapability() (*v1alpha1.ProvisionerCapabilitySpec, error)
}

type plugin struct {
	conn    *grpc.ClientConn
	timeout time.Duration
}

func NewPlugin(conn *grpc.ClientConn, timeout time.Duration) PluginHandler {
	return &plugin{
		conn, timeout,
	}
}

func (p *plugin) GetPluginInfo() (*v1alpha1.ProvisionerCapabilitySpecPluginInfo, error) {
	client := csi.NewIdentityClient(p.conn)

	req := csi.GetPluginInfoRequest{}
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	rsp, err := client.GetPluginInfo(ctx, &req)
	if err != nil {
		return nil, err
	}

	name, ver := rsp.GetName(), rsp.GetVendorVersion()
	return &v1alpha1.ProvisionerCapabilitySpecPluginInfo{
		Name:    name,
		Version: ver,
	}, nil
}

func (p *plugin) GetIdentityCapability() (topo bool, expand v1alpha1.ExpandMode, err error) {
	client := csi.NewIdentityClient(p.conn)

	req := csi.GetPluginCapabilitiesRequest{}
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	rsp, err := client.GetPluginCapabilities(ctx, &req)
	if err != nil {
		return false, v1alpha1.ExpandModeUnknown, err
	}
	for _, cap := range rsp.GetCapabilities() {
		if cap == nil {
			continue
		}
		srv := cap.GetService()
		if srv != nil && srv.GetType() == csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS {
			topo = true
		}
		exp := cap.GetVolumeExpansion()
		if exp != nil {
			switch exp.GetType() {
			case csi.PluginCapability_VolumeExpansion_UNKNOWN:
				expand = v1alpha1.ExpandModeUnknown
			case csi.PluginCapability_VolumeExpansion_ONLINE:
				expand = v1alpha1.ExpandModeOnline
			case csi.PluginCapability_VolumeExpansion_OFFLINE:
				expand = v1alpha1.ExpandModeOffline
			default:
				expand = v1alpha1.ExpandModeUnknown
			}
		}
	}
	return topo, expand, nil
}

type NodeCapabilitySet map[csi.NodeServiceCapability_RPC_Type]bool

func (p *plugin) GetNodeCapabilities() (NodeCapabilitySet, error) {
	client := csi.NewNodeClient(p.conn)
	req := csi.NodeGetCapabilitiesRequest{}
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	rsp, err := client.NodeGetCapabilities(ctx, &req)
	if err != nil {
		return nil, err
	}

	caps := NodeCapabilitySet{}
	for _, cap := range rsp.GetCapabilities() {
		if cap == nil {
			continue
		}
		rpc := cap.GetRpc()
		if rpc == nil {
			continue
		}
		t := rpc.GetType()
		caps[t] = true
	}
	return caps, nil
}

func (p *plugin) GetFullCapability() (*v1alpha1.ProvisionerCapabilitySpec, error) {
	info, err := p.GetPluginInfo()
	if err != nil {
		return nil, err
	}
	topology, expand, err := p.GetIdentityCapability()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	controllerCapSet, err := librpc.GetControllerCapabilities(ctx, p.conn)
	if err != nil {
		return nil, err
	}
	nodeCapSet, err := p.GetNodeCapabilities()
	if err != nil {
		return nil, err
	}
	return &v1alpha1.ProvisionerCapabilitySpec{
		PluginInfo: *info,
		Features: v1alpha1.ProvisionerCapabilitySpecFeatures{
			Topology: topology,
			Volume: v1alpha1.ProvisionerCapabilitySpecFeaturesVolume{
				Create: controllerCapSet[csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME],
				Attach: controllerCapSet[csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME],
				List:   controllerCapSet[csi.ControllerServiceCapability_RPC_LIST_VOLUMES],
				Clone:  controllerCapSet[csi.ControllerServiceCapability_RPC_CLONE_VOLUME],
				Stats:  nodeCapSet[csi.NodeServiceCapability_RPC_GET_VOLUME_STATS],
				Expand: expand,
			},
			Snapshot: v1alpha1.ProvisionerCapabilitySpecFeaturesSnapshot{
				Create: controllerCapSet[csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT],
				List:   controllerCapSet[csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS],
			},
		},
	}, nil
}
