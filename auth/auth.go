package auth

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	yaml "gopkg.in/yaml.v2"
)

type PasswordAuth struct {
	Username string
	Password string
}

type Auth struct {
	CaCert       string `yaml:"ca_cert"`
	ClientCert   string `yaml:"client_cert"`
	ClientKey    string `yaml:"client_key"`
	PasswordAuth string `yaml:"password_auth"`
	Username     string `yaml:"-"`
	Password     string `yaml:"-"`
}

func (auth *Auth) HasPassword() bool {
	return auth.Password != ""
}

func (auth *Auth) ResolvePassword() error {
	var a PasswordAuth

	if auth.ClientCert != "" {
		return nil
	}

	b, err := ioutil.ReadFile(auth.PasswordAuth)
	if err != nil {
		return errors.New(fmt.Sprintf("Error reading the password auth file: %s", err.Error()))
	}

	err = yaml.Unmarshal(b, &a)
	if err != nil {
		return errors.New(fmt.Sprintf("Error parsing the password auth file: %s", err.Error()))
	}

	auth.Username = a.Username
	auth.Password = a.Password

	return nil
}

func (auth *Auth) GetTlsConfigs() (*tls.Config, error) {
	tlsConf := &tls.Config{}

	//User credentials
	if auth.ClientCert != "" {
		certData, err := tls.LoadX509KeyPair(auth.ClientCert, auth.ClientKey)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to load user credentials: %s", err.Error()))
		}
		(*tlsConf).Certificates = []tls.Certificate{certData}
	}

	(*tlsConf).InsecureSkipVerify = false

	//CA cert
	if auth.CaCert != "" {
		caCertContent, err := ioutil.ReadFile(auth.CaCert)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to read root certificate file: %s", err.Error()))
		}
		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(caCertContent)
		if !ok {
			return nil, errors.New("Failed to parse root certificate authority")
		}
		(*tlsConf).RootCAs = roots
	}

	return tlsConf, nil
}