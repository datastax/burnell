package util

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"reflect"
	"strings"

	"unicode"

	"github.com/ghodss/yaml"
	"github.com/kafkaesque-io/burnell/src/icrypto"
)

// DefaultConfigFile - default config file
// it can be overwritten by env variable PULSAR_BEAM_CONFIG
const DefaultConfigFile = "../config/burnell.yml"

// Configuration - this server's configuration
type Configuration struct {
	PORT            string `json:"PORT"`
	ProxyURL        string `json:"ProxyURL"`
	AdminRestPrefix string `json:"AdminRestPrefix"`
	ClusterName     string `json:"ClusterName"`

	PulsarPublicKey  string `json:"PulsarPublicKey"`
	PulsarPrivateKey string `json:"PulsarPrivateKey"`
	SuperRoles       string `json:"SuperRoles"`

	PulsarToken string `json:"PulsarToken"`
	PulsarURL   string `json:"PulsarURL"`
	TrustStore  string `json:"TrustStore"`
	CertFile    string `json:"CertFile"`
	KeyFile     string `json:"KeyFile"`

	FederatedPromURL      string `json:"FederatedPromURL"`
	FederatedPromInterval string `json:"FederatedPromInterval"`

	TenantManagmentTopic string `json:"TenantManagmentTopic"`

	LogServerPort string `json:"LogServerPort"`
}

// Config - this server's configuration instance
var Config Configuration

// JWTAuth is the RSA key pair for sign and verify JWT
var JWTAuth *icrypto.RSAKeyPair

// ProxyURL is the destination URL for the proxy
var ProxyURL *url.URL

// AdminRestPrefix is the route prefix for proxy routing
var AdminRestPrefix string

// SuperRoles is super and admin roles for Pulsar
var SuperRoles []string

// Init initializes configuration
func Init() {
	configFile := AssignString(os.Getenv("BURNELL_CONFIG"), DefaultConfigFile)
	log.Printf("Configuration built from file - %s", configFile)
	ReadConfigFile(configFile)

	JWTAuth = icrypto.NewRSAKeyPair(Config.PulsarPrivateKey, Config.PulsarPublicKey)
	uri, err := url.ParseRequestURI(Config.ProxyURL)
	if err != nil {
		log.Fatal(err)
	}
	ProxyURL = uri
	AdminRestPrefix = Config.AdminRestPrefix

	for _, v := range strings.Split(Config.SuperRoles, ",") {
		SuperRoles = append(SuperRoles, strings.TrimSpace(v))
	}
}

// ReadConfigFile reads configuration file.
func ReadConfigFile(configFile string) {
	fileBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Printf("failed to load configuration file %s", configFile)
		panic(err)
	}

	if hasJSONPrefix(fileBytes) {
		err = json.Unmarshal(fileBytes, &Config)
		if err != nil {
			panic(err)
		}
	} else {
		err = yaml.Unmarshal(fileBytes, &Config)
		if err != nil {
			panic(err)
		}
	}

	// Next section allows env variable overwrites config file value
	fields := reflect.TypeOf(Config)
	// pointer to struct
	values := reflect.ValueOf(&Config)
	// struct
	st := values.Elem()
	for i := 0; i < fields.NumField(); i++ {
		field := fields.Field(i).Name
		envV := os.Getenv(field)
		if len(envV) > 0 {
			f := st.FieldByName(field)
			if f.IsValid() && f.CanSet() && f.Kind() == reflect.String {
				f.SetString(envV)
			}
		}
	}

	log.Println(Config)
}

//GetConfig returns a reference to the Configuration
func GetConfig() *Configuration {
	return &Config
}

var jsonPrefix = []byte("{")

func hasJSONPrefix(buf []byte) bool {
	return hasPrefix(buf, jsonPrefix)
}

// Return true if the first non-whitespace bytes in buf is prefix.
func hasPrefix(buf []byte, prefix []byte) bool {
	trim := bytes.TrimLeftFunc(buf, unicode.IsSpace)
	return bytes.HasPrefix(trim, prefix)
}
