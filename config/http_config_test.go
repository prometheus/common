// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build go1.8

package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	yaml "gopkg.in/yaml.v2"
)

const (
	TLSCAChainPath        = "testdata/tls-ca-chain.pem"
	ServerCertificatePath = "testdata/server.crt"
	ServerKeyPath         = "testdata/server.key"
	BarneyCertificatePath = "testdata/barney.crt"
	BarneyKeyNoPassPath   = "testdata/barney-no-pass.key"
	InvalidCA             = "testdata/barney-no-pass.key"
	WrongClientCertPath   = "testdata/self-signed-client.crt"
	WrongClientKeyPath    = "testdata/self-signed-client.key"
	EmptyFile             = "testdata/empty"
	MissingCA             = "missing/ca.crt"
	MissingCert           = "missing/cert.crt"
	MissingKey            = "missing/secret.key"

	ExpectedMessage        = "I'm here to serve you!!!"
	BearerToken            = "theanswertothegreatquestionoflifetheuniverseandeverythingisfortytwo"
	BearerTokenFile        = "testdata/bearer.token"
	MissingBearerTokenFile = "missing/bearer.token"
	ExpectedBearer         = "Bearer " + BearerToken
	ExpectedUsername       = "arthurdent"
	ExpectedPassword       = "42"
)

var invalidHTTPClientConfigs = []struct {
	httpClientConfigFile string
	errMsg               string
}{
	{
		httpClientConfigFile: "testdata/http.conf.bearer-token-and-file-set.bad.yml",
		errMsg:               "at most one of bearer_token & bearer_token_file must be configured",
	},
	{
		httpClientConfigFile: "testdata/http.conf.empty.bad.yml",
		errMsg:               "at most one of basic_auth, bearer_token & bearer_token_file must be configured",
	},
	{
		httpClientConfigFile: "testdata/http.conf.basic-auth.too-much.bad.yaml",
		errMsg:               "at most one of basic_auth password & password_file must be configured",
	},
}

func newTestServer(handler func(w http.ResponseWriter, r *http.Request)) (*httptest.Server, error) {
	testServer := httptest.NewUnstartedServer(http.HandlerFunc(handler))

	tlsCAChain, err := ioutil.ReadFile(TLSCAChainPath)
	if err != nil {
		return nil, fmt.Errorf("Can't read %s", TLSCAChainPath)
	}
	serverCertificate, err := tls.LoadX509KeyPair(ServerCertificatePath, ServerKeyPath)
	if err != nil {
		return nil, fmt.Errorf("Can't load X509 key pair %s - %s", ServerCertificatePath, ServerKeyPath)
	}

	rootCAs := x509.NewCertPool()
	rootCAs.AppendCertsFromPEM(tlsCAChain)

	testServer.TLS = &tls.Config{
		Certificates: make([]tls.Certificate, 1),
		RootCAs:      rootCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    rootCAs}
	testServer.TLS.Certificates[0] = serverCertificate
	testServer.TLS.BuildNameToCertificate()

	testServer.StartTLS()

	return testServer, nil
}

func TestNewClientFromConfig(t *testing.T) {
	var newClientValidConfig = []struct {
		clientConfig HTTPClientConfig
		handler      func(w http.ResponseWriter, r *http.Request)
	}{
		{
			clientConfig: HTTPClientConfig{
				TLSConfig: TLSConfig{
					CAFile:             "",
					CertFile:           BarneyCertificatePath,
					KeyFile:            BarneyKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: true},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, ExpectedMessage)
			},
		}, {
			clientConfig: HTTPClientConfig{
				TLSConfig: TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           BarneyCertificatePath,
					KeyFile:            BarneyKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, ExpectedMessage)
			},
		}, {
			clientConfig: HTTPClientConfig{
				TLSConfig: TLSConfig{
					CAData: `Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number: 2 (0x2)
    Signature Algorithm: sha1WithRSAEncryption
        Issuer: C=NO, O=Green AS, OU=Green Certificate Authority, CN=Green Root CA
        Validity
            Not Before: Jul 13 03:47:20 2017 GMT
            Not After : Jul 13 03:47:20 2027 GMT
        Subject: C=NO, O=Green AS, OU=Green Certificate Authority, CN=Green TLS CA
        Subject Public Key Info:
            Public Key Algorithm: rsaEncryption
                Public-Key: (2048 bit)
                Modulus:
                    00:b5:5a:b3:7a:7f:6a:5b:e9:ee:62:ee:4f:61:42:
                    79:93:06:bf:81:fc:9a:1f:b5:80:83:7c:b3:a6:94:
                    54:58:8a:b1:74:cb:c3:b8:3c:23:a8:69:1f:ca:2b:
                    af:be:97:ba:31:73:b5:b8:ce:d9:bf:bf:9a:7a:cf:
                    3a:64:51:83:c9:36:d2:f7:3b:3a:0e:4c:c7:66:2e:
                    bf:1a:df:ce:10:aa:3d:0f:19:74:03:7e:b5:10:bb:
                    e8:37:bd:62:f0:42:2d:df:3d:ca:70:50:10:17:ce:
                    a9:ec:55:8e:87:6f:ce:9a:04:36:14:96:cb:d1:a5:
                    48:d5:d2:87:02:62:93:4e:21:4a:ff:be:44:f1:d2:
                    7e:ed:74:da:c2:51:26:8e:03:a0:c2:bd:bd:5f:b0:
                    50:11:78:fd:ab:1d:04:86:6c:c1:8d:20:bd:05:5f:
                    51:67:c6:d3:07:95:92:2d:92:90:00:c6:9f:2d:dd:
                    36:5c:dc:78:10:7c:f6:68:39:1d:2c:e0:e1:26:64:
                    4f:36:34:66:a7:84:6a:90:15:3a:94:b7:79:b1:47:
                    f5:d2:51:95:54:bf:92:76:9a:b9:88:ee:63:f9:6c:
                    0d:38:c6:b6:1c:06:43:ed:24:1d:bb:6c:72:48:cc:
                    8c:f4:35:bc:43:fe:a6:96:4c:31:5f:82:0d:0d:20:
                    2a:3d
                Exponent: 65537 (0x10001)
        X509v3 extensions:
            X509v3 Key Usage: critical
                Certificate Sign, CRL Sign
            X509v3 Basic Constraints: critical
                CA:TRUE, pathlen:0
            X509v3 Subject Key Identifier: 
                AE:42:88:75:DD:05:A6:8E:48:7F:50:69:F9:B7:34:23:49:B8:B4:71
            X509v3 Authority Key Identifier: 
                keyid:60:93:53:2F:C7:CF:2A:D7:F3:09:28:F6:3C:AE:9C:50:EC:93:63:E5

            Authority Information Access: 
                CA Issuers - URI:http://green.no/ca/root-ca.cer

            X509v3 CRL Distribution Points: 

                Full Name:
                  URI:http://green.no/ca/root-ca.crl

    Signature Algorithm: sha1WithRSAEncryption
         15:a7:ac:d7:25:9e:2a:d4:d1:14:b4:99:38:3d:2f:73:61:2a:
         d9:b6:8b:13:ea:fe:db:78:d9:0a:6c:df:26:6e:c1:d5:4a:97:
         42:19:dd:97:05:03:e4:2b:fc:1e:1f:38:3c:4e:b0:3b:8c:38:
         ad:2b:65:fa:35:2d:81:8e:e0:f6:0a:89:4c:38:97:01:4b:9c:
         ac:4e:e1:55:17:ef:0a:ad:a7:eb:1e:4b:86:23:12:f1:52:69:
         cb:a3:8a:ce:fb:14:8b:86:d7:bb:81:5e:bd:2a:c7:a7:79:58:
         00:10:c0:db:ff:d4:a5:b9:19:74:b3:23:19:4a:1f:78:4b:a8:
         b6:f6:20:26:c1:69:f9:89:7f:b8:1c:3b:a2:f9:37:31:80:2c:
         b0:b6:2b:d2:84:44:d7:42:e4:e6:44:51:04:35:d9:1c:a4:48:
         c6:b7:35:de:f2:ae:da:4b:ba:c8:09:42:8d:ed:7a:81:dc:ed:
         9d:f0:de:6e:21:b9:01:1c:ad:64:3d:25:4c:91:94:f1:13:18:
         bb:89:e9:48:ac:05:73:07:c8:db:bd:69:8e:6f:02:9d:b0:18:
         c0:b9:e1:a8:b1:17:50:3d:ac:05:6e:6f:63:4f:b1:73:33:60:
         9a:77:d2:81:8a:01:38:43:e9:4c:3c:90:63:a4:99:4b:d2:1b:
         f9:1b:ec:ee
-----BEGIN CERTIFICATE-----
MIIECzCCAvOgAwIBAgIBAjANBgkqhkiG9w0BAQUFADBeMQswCQYDVQQGEwJOTzER
MA8GA1UECgwIR3JlZW4gQVMxJDAiBgNVBAsMG0dyZWVuIENlcnRpZmljYXRlIEF1
dGhvcml0eTEWMBQGA1UEAwwNR3JlZW4gUm9vdCBDQTAeFw0xNzA3MTMwMzQ3MjBa
Fw0yNzA3MTMwMzQ3MjBaMF0xCzAJBgNVBAYTAk5PMREwDwYDVQQKDAhHcmVlbiBB
UzEkMCIGA1UECwwbR3JlZW4gQ2VydGlmaWNhdGUgQXV0aG9yaXR5MRUwEwYDVQQD
DAxHcmVlbiBUTFMgQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC1
WrN6f2pb6e5i7k9hQnmTBr+B/JoftYCDfLOmlFRYirF0y8O4PCOoaR/KK6++l7ox
c7W4ztm/v5p6zzpkUYPJNtL3OzoOTMdmLr8a384Qqj0PGXQDfrUQu+g3vWLwQi3f
PcpwUBAXzqnsVY6Hb86aBDYUlsvRpUjV0ocCYpNOIUr/vkTx0n7tdNrCUSaOA6DC
vb1fsFAReP2rHQSGbMGNIL0FX1FnxtMHlZItkpAAxp8t3TZc3HgQfPZoOR0s4OEm
ZE82NGanhGqQFTqUt3mxR/XSUZVUv5J2mrmI7mP5bA04xrYcBkPtJB27bHJIzIz0
NbxD/qaWTDFfgg0NICo9AgMBAAGjgdQwgdEwDgYDVR0PAQH/BAQDAgEGMBIGA1Ud
EwEB/wQIMAYBAf8CAQAwHQYDVR0OBBYEFK5CiHXdBaaOSH9Qafm3NCNJuLRxMB8G
A1UdIwQYMBaAFGCTUy/HzyrX8wko9jyunFDsk2PlMDoGCCsGAQUFBwEBBC4wLDAq
BggrBgEFBQcwAoYeaHR0cDovL2dyZWVuLm5vL2NhL3Jvb3QtY2EuY2VyMC8GA1Ud
HwQoMCYwJKAioCCGHmh0dHA6Ly9ncmVlbi5uby9jYS9yb290LWNhLmNybDANBgkq
hkiG9w0BAQUFAAOCAQEAFaes1yWeKtTRFLSZOD0vc2Eq2baLE+r+23jZCmzfJm7B
1UqXQhndlwUD5Cv8Hh84PE6wO4w4rStl+jUtgY7g9gqJTDiXAUucrE7hVRfvCq2n
6x5LhiMS8VJpy6OKzvsUi4bXu4FevSrHp3lYABDA2//UpbkZdLMjGUofeEuotvYg
JsFp+Yl/uBw7ovk3MYAssLYr0oRE10Lk5kRRBDXZHKRIxrc13vKu2ku6yAlCje16
gdztnfDebiG5ARytZD0lTJGU8RMYu4npSKwFcwfI271pjm8CnbAYwLnhqLEXUD2s
BW5vY0+xczNgmnfSgYoBOEPpTDyQY6SZS9Ib+Rvs7g==
-----END CERTIFICATE-----
Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number: 1 (0x1)
    Signature Algorithm: sha1WithRSAEncryption
        Issuer: C=NO, O=Green AS, OU=Green Certificate Authority, CN=Green Root CA
        Validity
            Not Before: Jul 13 03:44:39 2017 GMT
            Not After : Dec 31 23:59:59 2030 GMT
        Subject: C=NO, O=Green AS, OU=Green Certificate Authority, CN=Green Root CA
        Subject Public Key Info:
            Public Key Algorithm: rsaEncryption
                Public-Key: (2048 bit)
                Modulus:
                    00:a7:e8:ed:de:d4:54:08:41:07:40:d5:c0:43:d6:
                    ab:d3:9e:21:87:c6:13:bf:a7:cf:3d:08:4f:c1:fe:
                    8f:e5:6c:c5:89:97:e5:27:75:26:c3:2a:73:2d:34:
                    7c:6f:35:8d:40:66:61:05:c0:eb:e9:b3:38:47:f8:
                    8b:26:35:2c:df:dc:24:31:fe:72:e3:87:10:d1:f7:
                    a0:57:b7:f3:b1:1a:fe:c7:4b:f8:7b:14:6d:73:08:
                    54:eb:63:3c:0c:ce:22:95:5f:3f:f2:6f:89:ae:63:
                    da:80:74:36:21:13:e8:91:01:58:77:cc:c2:f2:42:
                    bf:eb:b3:60:a7:21:ed:88:24:7f:eb:ff:07:41:9b:
                    93:c8:5f:6a:8e:a6:1a:15:3c:bc:e7:0d:fd:05:fd:
                    3c:c1:1c:1d:1f:57:2b:40:27:62:a1:7c:48:63:c1:
                    45:e7:2f:20:ed:92:1c:42:94:e4:58:70:7a:b6:d2:
                    85:c5:61:d8:cd:c6:37:6b:72:3b:7f:af:55:81:d6:
                    9d:dc:10:c9:d8:0e:81:e4:5e:40:13:2f:20:e8:6b:
                    46:81:ce:88:47:dd:38:71:3d:ef:21:cc:c0:67:cf:
                    0a:f4:e9:3f:a8:9d:26:25:2e:23:1e:a3:11:18:cb:
                    d1:70:1c:9e:7d:09:b1:a4:20:dc:95:15:1d:49:cf:
                    1b:ad
                Exponent: 65537 (0x10001)
        X509v3 extensions:
            X509v3 Key Usage: critical
                Certificate Sign, CRL Sign
            X509v3 Basic Constraints: critical
                CA:TRUE
            X509v3 Subject Key Identifier: 
                60:93:53:2F:C7:CF:2A:D7:F3:09:28:F6:3C:AE:9C:50:EC:93:63:E5
            X509v3 Authority Key Identifier: 
                keyid:60:93:53:2F:C7:CF:2A:D7:F3:09:28:F6:3C:AE:9C:50:EC:93:63:E5

    Signature Algorithm: sha1WithRSAEncryption
         a7:77:71:8b:1a:e5:5a:5b:87:54:08:bf:07:3e:cb:99:2f:dc:
         0e:8d:63:94:95:83:19:c9:92:82:d5:cb:5b:8f:1f:86:55:bc:
         70:01:1d:33:46:ec:99:de:6b:1f:c3:c2:7a:dd:ef:69:ab:96:
         58:ec:6c:6f:6c:70:82:71:8a:7f:f0:3b:80:90:d5:64:fa:80:
         27:b8:7b:50:69:98:4b:37:99:ad:bf:a2:5b:93:22:5e:96:44:
         3c:5a:cf:0c:f4:62:63:4a:6f:72:a7:f6:89:1d:09:26:3d:8f:
         a8:86:d4:b4:bc:dd:b3:38:ca:c0:59:16:8c:20:1f:89:35:12:
         b4:2d:c0:e9:de:93:e0:39:76:32:fc:80:db:da:44:26:fd:01:
         32:74:97:f8:44:ae:fe:05:b1:34:96:13:34:56:73:b4:93:a5:
         55:56:d1:01:51:9d:9c:55:e7:38:53:28:12:4e:38:72:0c:8f:
         bd:91:4c:45:48:3b:e1:0d:03:5f:58:40:c9:d3:a0:ac:b3:89:
         ce:af:27:8a:0f:ab:ec:72:4d:40:77:30:6b:36:fd:32:46:9f:
         ee:f9:c4:f5:17:06:0f:4b:d3:88:f5:a4:2f:3d:87:9e:f5:26:
         74:f0:c9:dc:cb:ad:d9:a7:8a:d3:71:15:00:d3:5d:9f:4c:59:
         3e:24:63:f5
-----BEGIN CERTIFICATE-----
MIIDnDCCAoSgAwIBAgIBATANBgkqhkiG9w0BAQUFADBeMQswCQYDVQQGEwJOTzER
MA8GA1UECgwIR3JlZW4gQVMxJDAiBgNVBAsMG0dyZWVuIENlcnRpZmljYXRlIEF1
dGhvcml0eTEWMBQGA1UEAwwNR3JlZW4gUm9vdCBDQTAgFw0xNzA3MTMwMzQ0Mzla
GA8yMDMwMTIzMTIzNTk1OVowXjELMAkGA1UEBhMCTk8xETAPBgNVBAoMCEdyZWVu
IEFTMSQwIgYDVQQLDBtHcmVlbiBDZXJ0aWZpY2F0ZSBBdXRob3JpdHkxFjAUBgNV
BAMMDUdyZWVuIFJvb3QgQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB
AQCn6O3e1FQIQQdA1cBD1qvTniGHxhO/p889CE/B/o/lbMWJl+UndSbDKnMtNHxv
NY1AZmEFwOvpszhH+IsmNSzf3CQx/nLjhxDR96BXt/OxGv7HS/h7FG1zCFTrYzwM
ziKVXz/yb4muY9qAdDYhE+iRAVh3zMLyQr/rs2CnIe2IJH/r/wdBm5PIX2qOphoV
PLznDf0F/TzBHB0fVytAJ2KhfEhjwUXnLyDtkhxClORYcHq20oXFYdjNxjdrcjt/
r1WB1p3cEMnYDoHkXkATLyDoa0aBzohH3ThxPe8hzMBnzwr06T+onSYlLiMeoxEY
y9FwHJ59CbGkINyVFR1JzxutAgMBAAGjYzBhMA4GA1UdDwEB/wQEAwIBBjAPBgNV
HRMBAf8EBTADAQH/MB0GA1UdDgQWBBRgk1Mvx88q1/MJKPY8rpxQ7JNj5TAfBgNV
HSMEGDAWgBRgk1Mvx88q1/MJKPY8rpxQ7JNj5TANBgkqhkiG9w0BAQUFAAOCAQEA
p3dxixrlWluHVAi/Bz7LmS/cDo1jlJWDGcmSgtXLW48fhlW8cAEdM0bsmd5rH8PC
et3vaauWWOxsb2xwgnGKf/A7gJDVZPqAJ7h7UGmYSzeZrb+iW5MiXpZEPFrPDPRi
Y0pvcqf2iR0JJj2PqIbUtLzdszjKwFkWjCAfiTUStC3A6d6T4Dl2MvyA29pEJv0B
MnSX+ESu/gWxNJYTNFZztJOlVVbRAVGdnFXnOFMoEk44cgyPvZFMRUg74Q0DX1hA
ydOgrLOJzq8nig+r7HJNQHcwazb9Mkaf7vnE9RcGD0vTiPWkLz2HnvUmdPDJ3Mut
2aeK03EVANNdn0xZPiRj9Q==
-----END CERTIFICATE-----
`,
					CAFile:             "",
					CertFile:           BarneyCertificatePath,
					KeyFile:            BarneyKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, ExpectedMessage)
			},
		}, {
			clientConfig: HTTPClientConfig{
				BearerToken: BearerToken,
				TLSConfig: TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           BarneyCertificatePath,
					KeyFile:            BarneyKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				bearer := r.Header.Get("Authorization")
				if bearer != ExpectedBearer {
					fmt.Fprintf(w, "The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
						ExpectedBearer, bearer)
				} else {
					fmt.Fprint(w, ExpectedMessage)
				}
			},
		}, {
			clientConfig: HTTPClientConfig{
				BearerTokenFile: BearerTokenFile,
				TLSConfig: TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           BarneyCertificatePath,
					KeyFile:            BarneyKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				bearer := r.Header.Get("Authorization")
				if bearer != ExpectedBearer {
					fmt.Fprintf(w, "The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
						ExpectedBearer, bearer)
				} else {
					fmt.Fprint(w, ExpectedMessage)
				}
			},
		}, {
			clientConfig: HTTPClientConfig{
				BasicAuth: &BasicAuth{
					Username: ExpectedUsername,
					Password: ExpectedPassword,
				},
				TLSConfig: TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           BarneyCertificatePath,
					KeyFile:            BarneyKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				username, password, ok := r.BasicAuth()
				if !ok {
					fmt.Fprintf(w, "The Authorization header wasn't set")
				} else if ExpectedUsername != username {
					fmt.Fprintf(w, "The expected username (%s) differs from the obtained username (%s).", ExpectedUsername, username)
				} else if ExpectedPassword != password {
					fmt.Fprintf(w, "The expected password (%s) differs from the obtained password (%s).", ExpectedPassword, password)
				} else {
					fmt.Fprint(w, ExpectedMessage)
				}
			},
		},
	}

	for _, validConfig := range newClientValidConfig {
		testServer, err := newTestServer(validConfig.handler)
		if err != nil {
			t.Fatal(err.Error())
		}
		defer testServer.Close()

		client, err := NewClientFromConfig(validConfig.clientConfig, "test")
		if err != nil {
			t.Errorf("Can't create a client from this config: %+v", validConfig.clientConfig)
			continue
		}
		response, err := client.Get(testServer.URL)
		if err != nil {
			t.Errorf("Can't connect to the test server using this config: %+v", validConfig.clientConfig)
			continue
		}

		message, err := ioutil.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Errorf("Can't read the server response body using this config: %+v", validConfig.clientConfig)
			continue
		}

		trimMessage := strings.TrimSpace(string(message))
		if ExpectedMessage != trimMessage {
			t.Errorf("The expected message (%s) differs from the obtained message (%s) using this config: %+v",
				ExpectedMessage, trimMessage, validConfig.clientConfig)
		}
	}
}

func TestNewClientFromInvalidConfig(t *testing.T) {
	var newClientInvalidConfig = []struct {
		clientConfig HTTPClientConfig
		errorMsg     string
	}{
		{
			clientConfig: HTTPClientConfig{
				TLSConfig: TLSConfig{
					CAFile:             MissingCA,
					InsecureSkipVerify: true},
			},
			errorMsg: fmt.Sprintf("unable to load specified CA cert %s:", MissingCA),
		},
		{
			clientConfig: HTTPClientConfig{
				TLSConfig: TLSConfig{
					CAFile:             InvalidCA,
					InsecureSkipVerify: true},
			},
			errorMsg: fmt.Sprintf("unable to use specified CA cert %s", InvalidCA),
		},
	}

	for _, invalidConfig := range newClientInvalidConfig {
		client, err := NewClientFromConfig(invalidConfig.clientConfig, "test")
		if client != nil {
			t.Errorf("A client instance was returned instead of nil using this config: %+v", invalidConfig.clientConfig)
		}
		if err == nil {
			t.Errorf("No error was returned using this config: %+v", invalidConfig.clientConfig)
		}
		if !strings.Contains(err.Error(), invalidConfig.errorMsg) {
			t.Errorf("Expected error %q does not contain %q", err.Error(), invalidConfig.errorMsg)
		}
	}
}

func TestMissingBearerAuthFile(t *testing.T) {
	cfg := HTTPClientConfig{
		BearerTokenFile: MissingBearerTokenFile,
		TLSConfig: TLSConfig{
			CAFile:             TLSCAChainPath,
			CertFile:           BarneyCertificatePath,
			KeyFile:            BarneyKeyNoPassPath,
			ServerName:         "",
			InsecureSkipVerify: false},
	}
	handler := func(w http.ResponseWriter, r *http.Request) {
		bearer := r.Header.Get("Authorization")
		if bearer != ExpectedBearer {
			fmt.Fprintf(w, "The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
				ExpectedBearer, bearer)
		} else {
			fmt.Fprint(w, ExpectedMessage)
		}
	}

	testServer, err := newTestServer(handler)
	if err != nil {
		t.Fatal(err.Error())
	}
	defer testServer.Close()

	client, err := NewClientFromConfig(cfg, "test")
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Get(testServer.URL)
	if err == nil {
		t.Fatal("No error is returned here")
	}

	if !strings.Contains(err.Error(), "unable to read bearer token file missing/bearer.token: open missing/bearer.token: no such file or directory") {
		t.Fatal("wrong error message being returned")
	}
}

func TestBearerAuthRoundTripper(t *testing.T) {
	const (
		newBearerToken = "goodbyeandthankyouforthefish"
	)

	fakeRoundTripper := NewRoundTripCheckRequest(func(req *http.Request) {
		bearer := req.Header.Get("Authorization")
		if bearer != ExpectedBearer {
			t.Errorf("The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
				ExpectedBearer, bearer)
		}
	}, nil, nil)

	// Normal flow.
	bearerAuthRoundTripper := NewBearerAuthRoundTripper(BearerToken, fakeRoundTripper)
	request, _ := http.NewRequest("GET", "/hitchhiker", nil)
	request.Header.Set("User-Agent", "Douglas Adams mind")
	bearerAuthRoundTripper.RoundTrip(request)

	// Should honor already Authorization header set.
	bearerAuthRoundTripperShouldNotModifyExistingAuthorization := NewBearerAuthRoundTripper(newBearerToken, fakeRoundTripper)
	request, _ = http.NewRequest("GET", "/hitchhiker", nil)
	request.Header.Set("Authorization", ExpectedBearer)
	bearerAuthRoundTripperShouldNotModifyExistingAuthorization.RoundTrip(request)
}

func TestBearerAuthFileRoundTripper(t *testing.T) {
	fakeRoundTripper := NewRoundTripCheckRequest(func(req *http.Request) {
		bearer := req.Header.Get("Authorization")
		if bearer != ExpectedBearer {
			t.Errorf("The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
				ExpectedBearer, bearer)
		}
	}, nil, nil)

	// Normal flow.
	bearerAuthRoundTripper := NewBearerAuthFileRoundTripper(BearerTokenFile, fakeRoundTripper)
	request, _ := http.NewRequest("GET", "/hitchhiker", nil)
	request.Header.Set("User-Agent", "Douglas Adams mind")
	bearerAuthRoundTripper.RoundTrip(request)

	// Should honor already Authorization header set.
	bearerAuthRoundTripperShouldNotModifyExistingAuthorization := NewBearerAuthFileRoundTripper(MissingBearerTokenFile, fakeRoundTripper)
	request, _ = http.NewRequest("GET", "/hitchhiker", nil)
	request.Header.Set("Authorization", ExpectedBearer)
	bearerAuthRoundTripperShouldNotModifyExistingAuthorization.RoundTrip(request)
}

func TestTLSConfig(t *testing.T) {
	configTLSConfig := TLSConfig{
		CAFile:             TLSCAChainPath,
		CertFile:           BarneyCertificatePath,
		KeyFile:            BarneyKeyNoPassPath,
		ServerName:         "localhost",
		InsecureSkipVerify: false}

	tlsCAChain, err := ioutil.ReadFile(TLSCAChainPath)
	if err != nil {
		t.Fatalf("Can't read the CA certificate chain (%s)",
			TLSCAChainPath)
	}
	rootCAs := x509.NewCertPool()
	rootCAs.AppendCertsFromPEM(tlsCAChain)

	expectedTLSConfig := &tls.Config{
		RootCAs:            rootCAs,
		ServerName:         configTLSConfig.ServerName,
		InsecureSkipVerify: configTLSConfig.InsecureSkipVerify}

	tlsConfig, err := NewTLSConfig(&configTLSConfig)
	if err != nil {
		t.Fatalf("Can't create a new TLS Config from a configuration (%s).", err)
	}

	barneyCertificate, err := tls.LoadX509KeyPair(BarneyCertificatePath, BarneyKeyNoPassPath)
	if err != nil {
		t.Fatalf("Can't load the client key pair ('%s' and '%s'). Reason: %s",
			BarneyCertificatePath, BarneyKeyNoPassPath, err)
	}
	cert, err := tlsConfig.GetClientCertificate(nil)
	if err != nil {
		t.Fatalf("unexpected error returned by tlsConfig.GetClientCertificate(): %s", err)
	}
	if !reflect.DeepEqual(cert, &barneyCertificate) {
		t.Fatalf("Unexpected client certificate result: \n\n%+v\n expected\n\n%+v", cert, barneyCertificate)
	}

	// non-nil functions are never equal.
	tlsConfig.GetClientCertificate = nil
	if !reflect.DeepEqual(tlsConfig, expectedTLSConfig) {
		t.Fatalf("Unexpected TLS Config result: \n\n%+v\n expected\n\n%+v", tlsConfig, expectedTLSConfig)
	}
}

func TestTLSConfigEmpty(t *testing.T) {
	configTLSConfig := TLSConfig{
		InsecureSkipVerify: true,
	}

	expectedTLSConfig := &tls.Config{
		InsecureSkipVerify: configTLSConfig.InsecureSkipVerify,
	}

	tlsConfig, err := NewTLSConfig(&configTLSConfig)
	if err != nil {
		t.Fatalf("Can't create a new TLS Config from a configuration (%s).", err)
	}

	if !reflect.DeepEqual(tlsConfig, expectedTLSConfig) {
		t.Fatalf("Unexpected TLS Config result: \n\n%+v\n expected\n\n%+v", tlsConfig, expectedTLSConfig)
	}
}

func TestTLSConfigInvalidCA(t *testing.T) {
	var invalidTLSConfig = []struct {
		configTLSConfig TLSConfig
		errorMessage    string
	}{
		{
			configTLSConfig: TLSConfig{
				CAFile:             MissingCA,
				CertFile:           "",
				KeyFile:            "",
				ServerName:         "",
				InsecureSkipVerify: false},
			errorMessage: fmt.Sprintf("unable to load specified CA cert %s:", MissingCA),
		}, {
			configTLSConfig: TLSConfig{
				CAFile:             "",
				CertFile:           MissingCert,
				KeyFile:            BarneyKeyNoPassPath,
				ServerName:         "",
				InsecureSkipVerify: false},
			errorMessage: fmt.Sprintf("unable to use specified client cert (%s) & key (%s):", MissingCert, BarneyKeyNoPassPath),
		}, {
			configTLSConfig: TLSConfig{
				CAFile:             "",
				CertFile:           BarneyCertificatePath,
				KeyFile:            MissingKey,
				ServerName:         "",
				InsecureSkipVerify: false},
			errorMessage: fmt.Sprintf("unable to use specified client cert (%s) & key (%s):", BarneyCertificatePath, MissingKey),
		},
	}

	for _, anInvalididTLSConfig := range invalidTLSConfig {
		tlsConfig, err := NewTLSConfig(&anInvalididTLSConfig.configTLSConfig)
		if tlsConfig != nil && err == nil {
			t.Errorf("The TLS Config could be created even with this %+v", anInvalididTLSConfig.configTLSConfig)
			continue
		}
		if !strings.Contains(err.Error(), anInvalididTLSConfig.errorMessage) {
			t.Errorf("The expected error should contain %s, but got %s", anInvalididTLSConfig.errorMessage, err)
		}
	}
}

func TestBasicAuthNoPassword(t *testing.T) {
	cfg, _, err := LoadHTTPConfigFile("testdata/http.conf.basic-auth.no-password.yaml")
	if err != nil {
		t.Fatalf("Error loading HTTP client config: %v", err)
	}
	client, err := NewClientFromConfig(*cfg, "test")
	if err != nil {
		t.Fatalf("Error creating HTTP Client: %v", err)
	}

	rt, ok := client.Transport.(*basicAuthRoundTripper)
	if !ok {
		t.Fatalf("Error casting to basic auth transport, %v", client.Transport)
	}

	if rt.username != "user" {
		t.Errorf("Bad HTTP client username: %s", rt.username)
	}
	if string(rt.password) != "" {
		t.Errorf("Expected empty HTTP client password: %s", rt.password)
	}
	if string(rt.passwordFile) != "" {
		t.Errorf("Expected empty HTTP client passwordFile: %s", rt.passwordFile)
	}
}

func TestBasicAuthNoUsername(t *testing.T) {
	cfg, _, err := LoadHTTPConfigFile("testdata/http.conf.basic-auth.no-username.yaml")
	if err != nil {
		t.Fatalf("Error loading HTTP client config: %v", err)
	}
	client, err := NewClientFromConfig(*cfg, "test")
	if err != nil {
		t.Fatalf("Error creating HTTP Client: %v", err)
	}

	rt, ok := client.Transport.(*basicAuthRoundTripper)
	if !ok {
		t.Fatalf("Error casting to basic auth transport, %v", client.Transport)
	}

	if rt.username != "" {
		t.Errorf("Got unexpected username: %s", rt.username)
	}
	if string(rt.password) != "secret" {
		t.Errorf("Unexpected HTTP client password: %s", string(rt.password))
	}
	if string(rt.passwordFile) != "" {
		t.Errorf("Expected empty HTTP client passwordFile: %s", rt.passwordFile)
	}
}

func TestBasicAuthPasswordFile(t *testing.T) {
	cfg, _, err := LoadHTTPConfigFile("testdata/http.conf.basic-auth.good.yaml")
	if err != nil {
		t.Fatalf("Error loading HTTP client config: %v", err)
	}
	client, err := NewClientFromConfig(*cfg, "test")
	if err != nil {
		t.Fatalf("Error creating HTTP Client: %v", err)
	}

	rt, ok := client.Transport.(*basicAuthRoundTripper)
	if !ok {
		t.Fatalf("Error casting to basic auth transport, %v", client.Transport)
	}

	if rt.username != "user" {
		t.Errorf("Bad HTTP client username: %s", rt.username)
	}
	if string(rt.password) != "" {
		t.Errorf("Bad HTTP client password: %s", rt.password)
	}
	if string(rt.passwordFile) != "testdata/basic-auth-password" {
		t.Errorf("Bad HTTP client passwordFile: %s", rt.passwordFile)
	}
}

func getCertificateBlobs(t *testing.T) map[string][]byte {
	files := []string{
		TLSCAChainPath,
		BarneyCertificatePath,
		BarneyKeyNoPassPath,
		ServerCertificatePath,
		ServerKeyPath,
		WrongClientCertPath,
		WrongClientKeyPath,
		EmptyFile,
	}
	bs := make(map[string][]byte, len(files)+1)
	for _, f := range files {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		bs[f] = b
	}

	return bs
}

func writeCertificate(bs map[string][]byte, src string, dst string) {
	b, ok := bs[src]
	if !ok {
		panic(fmt.Sprintf("Couldn't find %q in bs", src))
	}
	if err := ioutil.WriteFile(dst, b, 0664); err != nil {
		panic(err)
	}
}

func TestTLSRoundTripper(t *testing.T) {
	bs := getCertificateBlobs(t)

	tmpDir, err := ioutil.TempDir("", "tlsroundtripper")
	if err != nil {
		t.Fatal("Failed to create tmp dir", err)
	}
	defer os.RemoveAll(tmpDir)

	ca, cert, key := filepath.Join(tmpDir, "ca"), filepath.Join(tmpDir, "cert"), filepath.Join(tmpDir, "key")

	handler := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, ExpectedMessage)
	}
	testServer, err := newTestServer(handler)
	if err != nil {
		t.Fatal(err.Error())
	}
	defer testServer.Close()

	testCases := []struct {
		ca   string
		cert string
		key  string

		errMsg string
	}{
		{
			// Valid certs.
			ca:   TLSCAChainPath,
			cert: BarneyCertificatePath,
			key:  BarneyKeyNoPassPath,
		},
		{
			// CA not matching.
			ca:   BarneyCertificatePath,
			cert: BarneyCertificatePath,
			key:  BarneyKeyNoPassPath,

			errMsg: "certificate signed by unknown authority",
		},
		{
			// Invalid client cert+key.
			ca:   TLSCAChainPath,
			cert: WrongClientCertPath,
			key:  WrongClientKeyPath,

			errMsg: "remote error: tls",
		},
		{
			// CA file empty
			ca:   EmptyFile,
			cert: BarneyCertificatePath,
			key:  BarneyKeyNoPassPath,

			errMsg: "unable to use specified CA cert",
		},
		{
			// cert file empty
			ca:   TLSCAChainPath,
			cert: EmptyFile,
			key:  BarneyKeyNoPassPath,

			errMsg: "failed to find any PEM data in certificate input",
		},
		{
			// key file empty
			ca:   TLSCAChainPath,
			cert: BarneyCertificatePath,
			key:  EmptyFile,

			errMsg: "failed to find any PEM data in key input",
		},
		{
			// Valid certs again.
			ca:   TLSCAChainPath,
			cert: BarneyCertificatePath,
			key:  BarneyKeyNoPassPath,
		},
	}

	cfg := HTTPClientConfig{
		TLSConfig: TLSConfig{
			CAFile:             ca,
			CertFile:           cert,
			KeyFile:            key,
			InsecureSkipVerify: false},
	}

	var c *http.Client
	for i, tc := range testCases {
		tc := tc
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			writeCertificate(bs, tc.ca, ca)
			writeCertificate(bs, tc.cert, cert)
			writeCertificate(bs, tc.key, key)
			if c == nil {
				c, err = NewClientFromConfig(cfg, "test")
				if err != nil {
					t.Fatalf("Error creating HTTP Client: %v", err)
				}
			}

			req, err := http.NewRequest(http.MethodGet, testServer.URL, nil)
			if err != nil {
				t.Fatalf("Error creating HTTP request: %v", err)
			}
			r, err := c.Do(req)
			if len(tc.errMsg) > 0 {
				if err == nil {
					r.Body.Close()
					t.Fatalf("Could connect to the test server.")
				}
				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Fatalf("Expected error message to contain %q, got %q", tc.errMsg, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Can't connect to the test server")
			}

			b, err := ioutil.ReadAll(r.Body)
			r.Body.Close()
			if err != nil {
				t.Errorf("Can't read the server response body")
			}

			got := strings.TrimSpace(string(b))
			if ExpectedMessage != got {
				t.Errorf("The expected message %q differs from the obtained message %q", ExpectedMessage, got)
			}
		})
	}
}

func TestTLSRoundTripperRaces(t *testing.T) {
	bs := getCertificateBlobs(t)

	tmpDir, err := ioutil.TempDir("", "tlsroundtripper")
	if err != nil {
		t.Fatal("Failed to create tmp dir", err)
	}
	defer os.RemoveAll(tmpDir)

	ca, cert, key := filepath.Join(tmpDir, "ca"), filepath.Join(tmpDir, "cert"), filepath.Join(tmpDir, "key")

	handler := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, ExpectedMessage)
	}
	testServer, err := newTestServer(handler)
	if err != nil {
		t.Fatal(err.Error())
	}
	defer testServer.Close()

	cfg := HTTPClientConfig{
		TLSConfig: TLSConfig{
			CAFile:             ca,
			CertFile:           cert,
			KeyFile:            key,
			InsecureSkipVerify: false},
	}

	var c *http.Client
	writeCertificate(bs, TLSCAChainPath, ca)
	writeCertificate(bs, BarneyCertificatePath, cert)
	writeCertificate(bs, BarneyKeyNoPassPath, key)
	c, err = NewClientFromConfig(cfg, "test")
	if err != nil {
		t.Fatalf("Error creating HTTP Client: %v", err)
	}

	var wg sync.WaitGroup
	ch := make(chan struct{})
	var total, ok int64
	// Spawn 10 Go routines polling the server concurrently.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ch:
					return
				default:
					atomic.AddInt64(&total, 1)
					r, err := c.Get(testServer.URL)
					if err == nil {
						r.Body.Close()
						atomic.AddInt64(&ok, 1)
					}
				}
			}
		}()
	}

	// Change the CA file every 10ms for 1 second.
	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0
		for {
			tick := time.NewTicker(10 * time.Millisecond)
			<-tick.C
			if i%2 == 0 {
				writeCertificate(bs, BarneyCertificatePath, ca)
			} else {
				writeCertificate(bs, TLSCAChainPath, ca)
			}
			i++
			if i > 100 {
				close(ch)
				return
			}
		}
	}()

	wg.Wait()
	if ok == total {
		t.Fatalf("Expecting some requests to fail but got %d/%d successful requests", ok, total)
	}
}

func TestHideHTTPClientConfigSecrets(t *testing.T) {
	c, _, err := LoadHTTPConfigFile("testdata/http.conf.good.yml")
	if err != nil {
		t.Errorf("Error parsing %s: %s", "testdata/http.conf.good.yml", err)
	}

	// String method must not reveal authentication credentials.
	s := c.String()
	if strings.Contains(s, "mysecret") {
		t.Fatal("http client config's String method reveals authentication credentials.")
	}
}

func TestValidateHTTPConfig(t *testing.T) {
	cfg, _, err := LoadHTTPConfigFile("testdata/http.conf.good.yml")
	if err != nil {
		t.Errorf("Error loading HTTP client config: %v", err)
	}
	err = cfg.Validate()
	if err != nil {
		t.Fatalf("Error validating %s: %s", "testdata/http.conf.good.yml", err)
	}
}

func TestInvalidHTTPConfigs(t *testing.T) {
	for _, ee := range invalidHTTPClientConfigs {
		_, _, err := LoadHTTPConfigFile(ee.httpClientConfigFile)
		if err == nil {
			t.Error("Expected error with config but got none")
			continue
		}
		if !strings.Contains(err.Error(), ee.errMsg) {
			t.Errorf("Expected error for invalid HTTP client configuration to contain %q but got: %s", ee.errMsg, err)
		}
	}
}

// LoadHTTPConfig parses the YAML input s into a HTTPClientConfig.
func LoadHTTPConfig(s string) (*HTTPClientConfig, error) {
	cfg := &HTTPClientConfig{}
	err := yaml.UnmarshalStrict([]byte(s), cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadHTTPConfigFile parses the given YAML file into a HTTPClientConfig.
func LoadHTTPConfigFile(filename string) (*HTTPClientConfig, []byte, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, nil, err
	}
	cfg, err := LoadHTTPConfig(string(content))
	if err != nil {
		return nil, nil, err
	}
	return cfg, content, nil
}

type roundTrip struct {
	theResponse *http.Response
	theError    error
}

func (rt *roundTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt.theResponse, rt.theError
}

type roundTripCheckRequest struct {
	checkRequest func(*http.Request)
	roundTrip
}

func (rt *roundTripCheckRequest) RoundTrip(r *http.Request) (*http.Response, error) {
	rt.checkRequest(r)
	return rt.theResponse, rt.theError
}

// NewRoundTripCheckRequest creates a new instance of a type that implements http.RoundTripper,
// which before returning theResponse and theError, executes checkRequest against a http.Request.
func NewRoundTripCheckRequest(checkRequest func(*http.Request), theResponse *http.Response, theError error) http.RoundTripper {
	return &roundTripCheckRequest{
		checkRequest: checkRequest,
		roundTrip: roundTrip{
			theResponse: theResponse,
			theError:    theError}}
}
