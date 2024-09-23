package apollo

import (
	"fmt"
	"github.com/micro/go-micro/v2/config/source"

	"github.com/zouyx/agollo/v4"
	"github.com/zouyx/agollo/v4/env/config"
	"time"
)

type apolloSource struct {
	namespaceName string
	opts          source.Options
	client        *agollo.Client
}

func (a *apolloSource) String() string {
	return "apollo"
}

func (a *apolloSource) Read() (*source.ChangeSet, error) {
	c := a.client.GetConfig(a.namespaceName).GetCache()

	kv := map[string]string{}

	c.Range(func(key interface{}, value interface{}) bool {
		kv[key.(string)] = value.(string)
		return true
	})
	data, err := makeMap(a.opts.Encoder, kv)
	if err != nil {
		return nil, fmt.Errorf("error reading data: %v", err)
	}

	b, err := a.opts.Encoder.Encode(data)
	if err != nil {
		return nil, fmt.Errorf("error reading source: %v", err)
	}

	cs := &source.ChangeSet{
		Timestamp: time.Now(),
		Format:    a.opts.Encoder.String(),
		Source:    a.String(),
		Data:      b,
	}
	cs.Checksum = cs.Sum()
	return cs, nil
}

func (a *apolloSource) Watch() (source.Watcher, error) {
	watcher, err := newWatcher(a.String(), a.opts.Encoder)
	a.client.AddChangeListener(watcher)
	return watcher, err
}

func (a *apolloSource) Write(cs *source.ChangeSet) error {
	return nil
}

func NewSource(opts ...source.Option) source.Source {
	options := source.NewOptions(opts...)
	namespace, _ := options.Context.Value(namespaceName{}).(string)

	address, _ := options.Context.Value(addressKey{}).(string)

	secret, _ := options.Context.Value(secretKey{}).(string)

	backupConfigPath, ok := options.Context.Value(backupConfigPathKey{}).(string)
	if !ok {
		backupConfigPath = "./config"
	}

	cluster, ok := options.Context.Value(clusterKey{}).(string)
	if !ok {
		cluster = "dev"
	}

	appId, ok := options.Context.Value(appIdKey{}).(string)

	readyConfig := &config.AppConfig{
		IsBackupConfig:   true,
		BackupConfigPath: backupConfigPath,
		AppID:            appId,
		Cluster:          cluster,
		NamespaceName:    namespace,
		IP:               address,
		Secret:           secret,
	}
	c, err := agollo.StartWithConfig(func() (*config.AppConfig, error) {
		return readyConfig, nil
	})

	if err != nil {
		panic("Apollo config init fatal: " + err.Error())
	}
	return &apolloSource{
		opts:          options,
		namespaceName: namespace,
		client:        c,
	}
}
