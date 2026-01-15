// Package consul is the consul discovery.
package consul

import (
	"github.com/go-kratos/kratos/contrib/registry/consul/v2"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/hashicorp/consul/api"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/aide-family/goddess/discovery"
	discoveryV1 "github.com/aide-family/goddess/pkg/discovery/v1"
)

func init() {
	discovery.Register("consul", New)
}

func New(discoveryConfig *discoveryV1.Discovery) (registry.Discovery, error) {
	options := &discoveryV1.ConsulDiscovery{}
	if err := anypb.UnmarshalTo(discoveryConfig.Options, options, proto.UnmarshalOptions{Merge: true}); err != nil {
		return nil, err
	}
	c := api.DefaultConfig()
	c.Address = options.Address
	c.Token = options.Token
	client, err := api.NewClient(c)
	if err != nil {
		return nil, err
	}
	return consul.New(client), nil
}
