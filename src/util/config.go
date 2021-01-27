//
//  Copyright (c) 2021 Datastax, Inc.
//
//  Licensed to the Apache Software Foundation (ASF) under one
//  or more contributor license agreements.  See the NOTICE file
//  distributed with this work for additional information
//  regarding copyright ownership.  The ASF licenses this file
//  to you under the Apache License, Version 2.0 (the
//  "License"); you may not use this file except in compliance
//  with the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing,
//  software distributed under the License is distributed on an
//  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
//  KIND, either express or implied.  See the License for the
//  specific language governing permissions and limitations
//  under the License.
//

package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"reflect"
	"strings"

	"unicode"

	"github.com/apex/log"
	"github.com/datastax/burnell/src/icrypto"
	"github.com/ghodss/yaml"
)

// DefaultConfigFile - default config file
// it can be overwritten by env variable PULSAR_BEAM_CONFIG
const DefaultConfigFile = "../config/burnell.yml"

// DummySuperRole is used for all authorization needs when authentication is disabled
const DummySuperRole = "DummySuperRole"

// Configuration - this server's configuration
type Configuration struct {
	LogLevel             string `json:"logLevel"`
	PORT                 string `json:"PORT"`
	WebsocketURL         string `json:"WebsocketURL"`
	BrokerProxyURL       string `json:"BrokerProxyURL"`
	FunctionProxyURL     string `json:"FunctionProxyURL"`
	AdminRestPrefix      string `json:"AdminRestPrefix"`
	ClusterName          string `json:"ClusterName"`
	PulsarNamespace      string `json:"PulsarNamespace"`
	PrivateKeySecretName string `json:"PrivateKeySecretName"`
	PublicKeySecretName  string `json:"PublicKeySecretName"`

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
	PulsarBeamTopic      string `json:"PulsarBeamTopic"`

	LogServerPort string `json:"LogServerPort"`
}

// Config - this server's configuration instance
var Config Configuration

// JWTAuth is the RSA key pair for sign and verify JWT
var JWTAuth *icrypto.RSAKeyPair

// BrokerProxyURL is the destination URL for the broker
var BrokerProxyURL *url.URL

// FunctionProxyURL is the destination URL for the function
var FunctionProxyURL *url.URL

// AdminRestPrefix is the route prefix for proxy routing
var AdminRestPrefix string

// SuperRoles is super and admin roles for Pulsar
var SuperRoles []string

// Init initializes configuration
func Init(mode *string) {
	configFile := AssignString(os.Getenv("BURNELL_CONFIG"), DefaultConfigFile)
	ReadConfigFile(configFile)

	log.SetLevel(logLevel(Config.LogLevel))
	log.Warnf("Configuration built from file - %s", configFile)
	if IsInitializer(mode) || IsHealer(mode) {
		return
	}
	var err error
	if IsPulsarJWTEnabled() {
		JWTAuth, err = icrypto.LoadRSAKeyPair(Config.PulsarPrivateKey, Config.PulsarPublicKey)
		if err != nil {
			panic(err)
		}
	}
	BrokerProxyURL, err = url.ParseRequestURI(Config.BrokerProxyURL)
	if err != nil {
		panic(err)
	}
	FunctionProxyURL, err = url.ParseRequestURI(Config.FunctionProxyURL)
	if err != nil {
		panic(err)
	}
	AdminRestPrefix = Config.AdminRestPrefix
}

// ReadConfigFile reads configuration file.
func ReadConfigFile(configFile string) {
	fileBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		fmt.Printf("failed to load configuration file %s", configFile)
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
		f := st.FieldByName(field)

		if f.Kind() == reflect.String {
			envV := os.Getenv(field)
			if len(envV) > 0 && f.IsValid() && f.CanSet() {
				f.SetString(strings.TrimSuffix(envV, "\n")) // ensure no \n at the end of line that was introduced by loading k8s secrete file
			}
			os.Setenv(field, f.String())
		}
	}

	if IsPulsarJWTEnabled() {
		SuperRoles = []string{}
		for _, v := range strings.Split(Config.SuperRoles, ",") {
			SuperRoles = append(SuperRoles, strings.TrimSpace(v))
		}
	} else {
		SuperRoles = []string{DummySuperRole}
	}

	log.Infof("configuration loaded is %v", Config)
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

func logLevel(level string) log.Level {
	switch strings.TrimSpace(strings.ToLower(level)) {
	case "debug":
		return log.DebugLevel
	case "warn":
		return log.WarnLevel
	case "error":
		return log.ErrorLevel
	case "fatal":
		return log.FatalLevel
	default:
		return log.InfoLevel
	}
}

// IsPulsarJWTEnabled evaluates if features related to Pulsar JWT is enabled or not
// features include validate and generate Pulsar JWT, role based authorization
func IsPulsarJWTEnabled() bool {
	c := GetConfig()
	if c.PulsarPublicKey == c.PulsarPrivateKey && c.PulsarPrivateKey == c.SuperRoles {
		return false
	}
	return true
}

// IsStatsMode returns if the burnell is running stats mode that collects and generates tenant stats only
func IsStatsMode() bool {
	c := GetConfig()
	return c.FederatedPromInterval != ""
}
