package workflow

import (
	"fmt"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/dgrijalva/jwt-go"
	"github.com/kafkaesque-io/burnell/src/icrypto"
	"github.com/kafkaesque-io/burnell/src/k8s"
	"github.com/kafkaesque-io/burnell/src/util"
)

// StepStatus is the k8s Pulsar cluster runtime status
type StepStatus int

const (
	// Created the cluster is stopped
	Created StepStatus = iota

	// InProgress the cluster
	InProgress

	// Succeeded is
	Succeeded

	// Failed is
	Failed
)

// Step is the each step in the workflow
type Step struct {
	Name        string     `json:"name"`
	Status      StepStatus `json:"status"`
	ErrorString string     `json:"error"`
}

// PulsarClusterStatusCode is the k8s Pulsar cluster runtime status
type PulsarClusterStatusCode int

const (
	// Stopped the cluster is stopped
	Stopped PulsarClusterStatusCode = iota

	// Configuring the cluster
	Configuring

	// Starting is the cluster is starting
	Starting

	// Started is the cluster has started
	Started

	// Deleted is the cluster has been deleted
	Deleted

	// NamespaceCreated is the status when the namespace is created
	NamespaceCreated
)

const (
	pulsarNamespaceSuffix      = "pulsar"
	certManagerNamespaceSuffix = "cert-manager"
	monitoringNamespaceSuffix  = "monitoring"

	// JWTFileExtension is
	JWTFileExtension = ".jwt"

	tokenPrivateKey = "token-private-key"
	tokenPublicKey  = "token-public-key"

	defaultPrivateKeyName = "my-private.key"
	defaultPublicKeyName  = "my-public.key"
)

var defaultRoleString = []string{"admin", "proxy", "superuser", "websocket"}

// Cluster is k8s Pulsar cluster
type Cluster struct {
	ClusterName          string `json:"clusterName"`
	KeysAndJWT           Step   `json:"keysAndJwt"`
	K8sNamespaces        Step   `json:"k8sNamespaces"`
	PulsarNamespace      string
	CertManagerNamespace string
	MonitoringNamespace  string
	SuperRoles           []string
	PrivateKeyFileName   string
	PublicKeyFileName    string
	Status               PulsarClusterStatusCode
	l                    *log.Entry
	KeysJWTs
}

// KeysJWTs is the keys and token
type KeysJWTs struct {
	KeyManager         *icrypto.RSAKeyPair
	PrivateKeyFilePath string
	PublicKeyFilePath  string
	PulsarJWTs         map[string]string // key is subject, value is pulsar token
	l                  *log.Entry
}

// NewCluster creates a new cluster
func NewCluster(clusterName, k8sNamespace string) *Cluster {
	cfg := util.GetConfig()
	return &Cluster{
		ClusterName:        clusterName,
		PulsarNamespace:    k8sNamespace,
		PrivateKeyFileName: util.AssignString(cfg.PrivateKeySecretName, defaultPrivateKeyName),
		PublicKeyFileName:  util.AssignString(cfg.PublicKeySecretName, defaultPublicKeyName),
		l: log.WithFields(log.Fields{
			"cluster": clusterName,
		}),
		// CertManagerNamespace: namespace(name, certManagerNamespaceSuffix),
		// MonitoringNamespace:  namespace(name, monitoringNamespaceSuffix),
	}
}

// Create creates all new keys and JWTs
func (m *Cluster) Create() error {
	m.l.Infof("cluster %v", m)
	// 1. create local RSA keys and tokens
	rsaKey, err := icrypto.NewRSAKeyPair()
	if err != nil {
		return err
	}
	keysAndJWTs := KeysJWTs{
		KeyManager: rsaKey,
		PulsarJWTs: make(map[string]string),
		l: log.WithFields(log.Fields{
			"account": m.ClusterName,
		}),
	}
	m.l.Infof("RSA public and private keys are created")

	// 2. create k8s cluster namespace
	k8sNamespace, err := k8s.LocalClient.CreatePulsarNamespace(m.PulsarNamespace)
	if k8sNamespace == "" {
		m.K8sNamespaces.Status = Failed
		return err
	}
	m.K8sNamespaces.Status = Succeeded
	m.l.Infof("kubernetes namespace %s is created", m.PulsarNamespace)

	// 3. create and upload keys and jwt
	if err := keysAndJWTs.Create(k8sNamespace, m.PrivateKeyFileName, m.PublicKeyFileName); err != nil {
		m.KeysAndJWT.Status = Failed
		return err
	}
	m.KeysAndJWT.Status = Succeeded
	m.l.Infof("keys and tokens are created and exported to kubernets secrets")
	return nil
}

// Healer monitors and repairs any missing keys and jwt
func (m *Cluster) Healer() error {
	m.l.Infof("cluster %v", m)
	k8sNamespace, err := k8s.LocalClient.CreatePulsarNamespace(m.PulsarNamespace)
	if k8sNamespace == "" {
		m.K8sNamespaces.Status = Failed
		return err
	}
	m.K8sNamespaces.Status = Succeeded
	m.l.Infof("kubernetes namespace %s is verified", m.PulsarNamespace)

	privateKey, err := getKeyFromSecret(m.PulsarNamespace, tokenPrivateKey, m.PrivateKeyFileName)
	if err != nil {
		log.Errorf("error to get secret %s under namespace %s, err %v", tokenPrivateKey, m.PulsarNamespace, err)
		return m.Create()
	}

	publicKey, err := getKeyFromSecret(m.PulsarNamespace, tokenPublicKey, m.PublicKeyFileName)
	if err != nil {
		log.Errorf("error to get secret %s under namespace %s, err %v", tokenPublicKey, m.PulsarNamespace, err)
		return m.Create()
	}
	m.l.Infof("private and public keys verified under namespace %s", m.PulsarNamespace)

	rsaKey, err := icrypto.LoadRSAKeyPairFromBase64(privateKey, publicKey)
	if err != nil {
		return err
	}
	keysAndJWTs := KeysJWTs{
		KeyManager: rsaKey,
		PulsarJWTs: make(map[string]string),
		l: log.WithFields(log.Fields{
			"account": m.ClusterName,
		}),
	}
	m.l.Infof("RSA public and private keys are fetched")

	// create and upload keys and jwt
	if err := keysAndJWTs.Repair(k8sNamespace, m.ClusterName); err != nil {
		m.KeysAndJWT.Status = Failed
		return err
	}
	m.KeysAndJWT.Status = Succeeded
	m.l.Infof("keys and tokens are verified")

	return nil
}

// Create creates private and public keys and admin superuser jwt
func (kj *KeysJWTs) Create(k8sNamespace, privateKeyName, publicKeyName string) error {
	k8s.LocalClient.CreateSecret(k8sNamespace, "token-private-key",
		map[string][]byte{
			privateKeyName: kj.KeyManager.PrivateKeyPKCS8Bytes,
		})
	k8s.LocalClient.CreateSecret(k8sNamespace, "token-public-key",
		map[string][]byte{
			publicKeyName: kj.KeyManager.PublicKeyPKIXBytes,
		})
	kj.l.Infof("successfully exported token private and public key as k8s secrets under namesapce %s", k8sNamespace)

	for _, v := range getAdminRoles() {
		role := strings.TrimSpace(v)
		tokenString, err := kj.KeyManager.GenerateToken(role, 0, jwt.SigningMethodRS256)
		if err != nil {
			return err
		}
		kj.PulsarJWTs[role] = tokenString

		// encode token and upload to k8s secrets
		k8sSecretName := "token-" + role
		data := map[string][]byte{
			role + JWTFileExtension: []byte(tokenString),
		}
		k8s.LocalClient.CreateSecret(k8sNamespace, k8sSecretName, data)
		kj.l.Infof("successfully exported %s token to secret %s under k8s namesapce %s", role, k8sSecretName, k8sNamespace)
	}

	return nil
}

// Repair creates new keys or JWTs if any one of them are missing
func (kj *KeysJWTs) Repair(k8sNamespace, clusterName string) error {
	for _, v := range getAdminRoles() {
		role := strings.TrimSpace(v)
		tokenString, err := kj.KeyManager.GenerateToken(role, 0, jwt.SigningMethodRS256)
		if err != nil {
			return err
		}
		kj.PulsarJWTs[role] = tokenString

		k8sSecretName := "token-" + role
		_, err = k8s.LocalClient.GetSecret(k8sNamespace, k8sSecretName)
		if err != nil {
			data := map[string][]byte{
				role + JWTFileExtension: []byte(tokenString),
			}
			err = k8s.LocalClient.CreateSecret(k8sNamespace, k8sSecretName, data)
			if err != nil {
				kj.l.Errorf("failed to create secret %s under namespace %s err %v", k8sSecretName, k8sNamespace, err)
			} else {
				kj.l.Infof("create secret %s under namespace %s", k8sSecretName, k8sNamespace)
			}
		}
	}

	return nil
}

// getKeyFromSecret gets either public or private keys from the secret
func getKeyFromSecret(k8sNamespace, secretName, secreteKey string) ([]byte, error) {
	secrets, err := k8s.LocalClient.GetSecret(k8sNamespace, secretName)
	if err != nil {
		return []byte{}, err //not found
	}

	for k, v := range secrets {
		if k == secreteKey {
			// return the first matched
			return v, nil
		}
	}
	return []byte{}, fmt.Errorf("no key found")
}

// ConfigKeysJWTs is the workflow to manage token public and private keys and JWTs
func ConfigKeysJWTs(setupOnce bool) {
	cfg := util.GetConfig()
	k8s.Init()
	// use a default namespace to prevent accidentally write to pulsar namespace
	pulsarNs := util.AssignString(cfg.PulsarNamespace, "burnell-ns")
	c := NewCluster(cfg.ClusterName, pulsarNs)
	if setupOnce {
		err := c.Healer()
		if err != nil {
			log.Errorf("keys-jwt create keys and jwts failure err %v", err)
		}
		return
	}

	interval := 5 * time.Minute
	go func(clusterName, pulsarNamespace string) {
		ticker := time.NewTicker(interval)
		for {
			select {
			case <-ticker.C:
				log.Infof("monitor and repair keys and jwts under namespace %s cluster %s", pulsarNamespace, clusterName)
				err := c.Healer()
				if err != nil {
					log.Errorf("keys-jwt repair failure err %v", err)
				}
			}
		}
	}(cfg.ClusterName, pulsarNs)

}

func getAdminRoles() []string {
	roleStr := util.GetConfig().SuperRoles
	if roleStr == "" {
		return defaultRoleString
	}
	roles := []string{}
	for _, v := range strings.Split(roleStr, ",") {
		roles = append(roles, strings.TrimSpace(v))
	}
	return roles
}
