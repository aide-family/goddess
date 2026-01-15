// Package etcd is the etcd discovery.
package etcd

import (
	"github.com/aide-family/magicbox/strutil"
	"github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	"github.com/go-kratos/kratos/v2/registry"
	clientV3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/aide-family/goddess/discovery"
	discoveryV1 "github.com/aide-family/goddess/pkg/discovery/v1"
)

func init() {
	discovery.Register("etcd", New)
}

func New(discoveryConfig *discoveryV1.Discovery) (registry.Discovery, error) {
	options := &discoveryV1.ETCDDiscovery{}
	if err := anypb.UnmarshalTo(discoveryConfig.Options, options, proto.UnmarshalOptions{Merge: true}); err != nil {
		return nil, err
	}
	client, err := clientV3.New(clientV3.Config{
		Endpoints:   strutil.SplitSkipEmpty(options.Endpoints, ","),
		Username:    options.Username,
		Password:    options.Password,
		DialTimeout: options.DialTimeout.AsDuration(),
	})
	if err != nil {
		return nil, err
	}
	return etcd.New(client), nil
}
