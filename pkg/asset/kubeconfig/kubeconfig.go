package kubeconfig

import (
	"fmt"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/openshift/installer/pkg/asset"
	"github.com/openshift/installer/pkg/asset/installconfig"
	"github.com/openshift/installer/pkg/asset/tls"
	clientcmd "k8s.io/client-go/tools/clientcmd/api/v1"
)

const (
	// KubeconfigUserNameAdmin is the user name of the admin kubeconfig.
	KubeconfigUserNameAdmin = "admin"
	// KubeconfigUserNameKubelet is the user name of the kubelet kubeconfig.
	KubeconfigUserNameKubelet = "kubelet"
)

// Kubeconfig implements the asset.Asset interface that generates
// the admin kubeconfig and kubelet kubeconfig.
type Kubeconfig struct {
	rootDir       string
	userName      string // admin or kubelet.
	rootCA        asset.Asset
	certKey       asset.Asset
	installConfig asset.Asset
}

var _ asset.Asset = (*Kubeconfig)(nil)

// Dependencies returns the dependency of the kubeconfig.
func (k *Kubeconfig) Dependencies() []asset.Asset {
	return []asset.Asset{
		k.rootCA,
		k.certKey,
		k.installConfig,
	}
}

// Generate generates the kubeconfig.
func (k *Kubeconfig) Generate(parents map[asset.Asset]*asset.State) (*asset.State, error) {
	var err error

	caCertData, err := asset.GetDataByFilename(k.rootCA, parents, tls.RootCACertName)
	if err != nil {
		return nil, err
	}

	var keyFilename, certFilename string
	switch k.userName {
	case KubeconfigUserNameAdmin:
		keyFilename, certFilename = tls.AdminKeyName, tls.AdminCertName
	case KubeconfigUserNameKubelet:
		keyFilename, certFilename = tls.KubeletKeyName, tls.KubeletCertName
	}
	clientKeyData, err := asset.GetDataByFilename(k.certKey, parents, keyFilename)
	if err != nil {
		return nil, err
	}
	clientCertData, err := asset.GetDataByFilename(k.certKey, parents, certFilename)
	if err != nil {
		return nil, err
	}
	installConfig, err := installconfig.GetInstallConfig(k.installConfig, parents)
	if err != nil {
		return nil, err
	}

	kubeconfig := clientcmd.Config{
		Clusters: []clientcmd.NamedCluster{
			{
				Name: installConfig.Name,
				Cluster: clientcmd.Cluster{
					Server: fmt.Sprintf("https://%s-api.%s:6443", installConfig.Name, installConfig.BaseDomain),
					CertificateAuthorityData: caCertData,
				},
			},
		},
		AuthInfos: []clientcmd.NamedAuthInfo{
			{
				Name: k.userName,
				AuthInfo: clientcmd.AuthInfo{
					ClientCertificateData: clientCertData,
					ClientKeyData:         clientKeyData,
				},
			},
		},
		Contexts: []clientcmd.NamedContext{
			{
				Name: k.userName,
				Context: clientcmd.Context{
					Cluster:  installConfig.Name,
					AuthInfo: k.userName,
				},
			},
		},
		CurrentContext: k.userName,
	}

	data, err := yaml.Marshal(kubeconfig)
	if err != nil {
		return nil, err
	}

	st := &asset.State{
		Contents: []asset.Content{
			{
				// E.g. generated/auth/kubeconfig-admin.
				Name: filepath.Join(k.rootDir, "auth", fmt.Sprintf("kubeconfig-%s", k.userName)),
				Data: data,
			},
		},
	}

	if err := st.PersistToFile(); err != nil {
		return nil, err
	}

	return st, nil
}
