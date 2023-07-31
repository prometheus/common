// Copyright 2020 The Prometheus-operator Authors
// Copyright 2022 The Prometheus Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build ignore
// +build ignore

// Program generating TLS certificates and keys for the tests.
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"time"
)

const (
	validityPeriod = 50 * 365 * 24 * time.Hour
)

func EncodeCertificate(w io.Writer, cert *x509.Certificate) error {
	return pem.Encode(w, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}

func EncodeKey(w io.Writer, priv *rsa.PrivateKey) error {
	b, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %v", err)
	}

	return pem.Encode(w, &pem.Block{Type: "PRIVATE KEY", Bytes: b})
}

var serialNumber *big.Int

func init() {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)

	var err error
	serialNumber, err = rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		panic(fmt.Errorf("failed to generate  serial number: %v", err))
	}
}

func SerialNumber() *big.Int {
	var serial big.Int

	serial.Set(serialNumber)
	serialNumber.Add(&serial, big.NewInt(1))

	return &serial

}

func GenerateCertificateAuthority(commonName string, parentCert *x509.Certificate, parentKey *rsa.PrivateKey) (*x509.Certificate, *rsa.PrivateKey, error) {
	now := time.Now()

	caKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate CA private key: %v", err)
	}

	caCert := &x509.Certificate{
		SerialNumber: SerialNumber(),
		Subject: pkix.Name{
			Country:            []string{"US"},
			Organization:       []string{"Prometheus"},
			OrganizationalUnit: []string{"Prometheus Certificate Authority"},
			CommonName:         commonName,
		},
		NotBefore:             now,
		NotAfter:              now.Add(validityPeriod),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		BasicConstraintsValid: true,
	}

	if parentCert == nil && parentKey == nil {
		parentCert = caCert
		parentKey = caKey
	}

	b, err := x509.CreateCertificate(rand.Reader, caCert, parentCert, &caKey.PublicKey, parentKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create CA certificate: %v", err)
	}

	caCert, err = x509.ParseCertificate(b)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode CA certificate: %v", err)
	}

	return caCert, caKey, nil
}

func GenerateCertificate(caCert *x509.Certificate, caKey *rsa.PrivateKey, server bool, name string, ipAddresses ...net.IP) (*x509.Certificate, *rsa.PrivateKey, error) {
	now := time.Now()

	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %v", err)
	}

	cert := &x509.Certificate{
		SerialNumber: SerialNumber(),
		Subject: pkix.Name{
			Country:      []string{"US"},
			Organization: []string{"Prometheus"},
			CommonName:   name,
		},
		NotBefore:             now,
		NotAfter:              now.Add(validityPeriod),
		KeyUsage:              x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
	}

	if server {
		cert.DNSNames = []string{name}
		cert.IPAddresses = ipAddresses
		cert.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}
	} else {
		cert.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}

	if caCert == nil && caKey == nil {
		caCert = cert
		caKey = key
	}

	b, err := x509.CreateCertificate(rand.Reader, cert, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %v", err)
	}

	cert, err = x509.ParseCertificate(b)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode certificate: %v", err)
	}

	return cert, key, nil
}

func writeCertificateAndKey(path string, cert *x509.Certificate, key *rsa.PrivateKey) error {
	var b bytes.Buffer

	if err := EncodeCertificate(&b, cert); err != nil {
		return err
	}

	if err := os.WriteFile(fmt.Sprintf("%s.crt", path), b.Bytes(), 0644); err != nil {
		return err
	}

	b.Reset()
	if err := EncodeKey(&b, key); err != nil {
		return err
	}

	if err := os.WriteFile(fmt.Sprintf("%s.key", path), b.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}

func GenerateCRL(cert *x509.Certificate, privateKey *rsa.PrivateKey, revokedCerts []pkix.RevokedCertificate, isExpired bool) ([]byte, error) {
	now := time.Now()

	next := now.AddDate(30, 0, 0)
	if isExpired {
		next = now
	}

	crl := &x509.RevocationList{
		SignatureAlgorithm:  x509.SHA256WithRSA,
		ThisUpdate:          now,
		NextUpdate:          next,
		RevokedCertificates: revokedCerts,
		Number:              big.NewInt(1),
		Issuer:              cert.Subject,
	}

	crlBytes, err := x509.CreateRevocationList(rand.Reader, crl, cert, privateKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create revocation list: %v", err)
	}

	return crlBytes, nil
}

func writeCRLs(filename string, crlData [][]byte) error {
	crlPemBytes := new(bytes.Buffer)
	for _, data := range crlData {
		crlPem := &pem.Block{
			Type:  "X509 CRL",
			Bytes: data,
		}
		err := pem.Encode(crlPemBytes, crlPem)
		if err != nil {
			return err
		}
	}

	if crlPemBytes == nil {
		return fmt.Errorf("empty CRL to write")
	}

	return os.WriteFile(filename, crlPemBytes.Bytes(), 0644)
}

func main() {
	log.Println("Generating root CA")
	rootCert, rootKey, err := GenerateCertificateAuthority("Prometheus Root CA", nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Generating CA")
	caCert, caKey, err := GenerateCertificateAuthority("Prometheus TLS CA", rootCert, rootKey)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Generating Irrelevant CA")
	irlvtCert, irlvtKey, err := GenerateCertificateAuthority("Prometheus TLS Irrelevant CA", nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Generating server certificate")
	cert, key, err := GenerateCertificate(caCert, caKey, true, "localhost", net.IPv4(127, 0, 0, 1), net.IPv4(127, 0, 0, 0))
	if err != nil {
		log.Fatal(err)
	}

	if err := writeCertificateAndKey("testdata/server", cert, key); err != nil {
		log.Fatal(err)
	}

	log.Println("Generating revoked server certificate")
	revokedCert, revokedKey, err := GenerateCertificate(caCert, caKey, true, "localhost", net.IPv4(127, 0, 0, 1), net.IPv4(127, 0, 0, 0))
	if err != nil {
		log.Fatal(err)
	}

	if err := writeCertificateAndKey("testdata/server_revoked", revokedCert, revokedKey); err != nil {
		log.Fatal(err)
	}

	log.Println("Generating client certificate")
	cert, key, err = GenerateCertificate(caCert, caKey, false, "localhost")
	if err != nil {
		log.Fatal(err)
	}

	if err := writeCertificateAndKey("testdata/client", cert, key); err != nil {
		log.Fatal(err)
	}

	log.Println("Generating self-signed client certificate")
	cert, key, err = GenerateCertificate(nil, nil, false, "localhost")
	if err != nil {
		log.Fatal(err)
	}

	if err := writeCertificateAndKey("testdata/self-signed-client", cert, key); err != nil {
		log.Fatal(err)
	}

	log.Println("Generating CA bundle")
	var b bytes.Buffer
	if err := EncodeCertificate(&b, caCert); err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("testdata/tls-ca-no-root.pem", b.Bytes(), 0644); err != nil {
		log.Fatal(err)
	}

	if err := EncodeCertificate(&b, rootCert); err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("testdata/tls-ca-chain.pem", b.Bytes(), 0644); err != nil {
		log.Fatal(err)
	}

	if err := EncodeCertificate(&b, irlvtCert); err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("testdata/tls-ca-chain-add-irlvt-ca.pem", b.Bytes(), 0644); err != nil {
		log.Fatal(err)
	}

	log.Println("Generating CRLs")
	crlProp_revokedCert := []pkix.RevokedCertificate{
		{
			SerialNumber:   revokedCert.SerialNumber,
			RevocationTime: time.Now(),
		},
	}

	crl_RevokedCert, err := GenerateCRL(caCert, caKey, crlProp_revokedCert, false)
	if err != nil {
		log.Fatal(err)
	}

	if err := writeCRLs("testdata/crl_cert_revoked.pem", [][]byte{crl_RevokedCert}); err != nil {
		log.Fatal(err)
	}

	crl_RevokedCert_expired, err := GenerateCRL(caCert, caKey, crlProp_revokedCert, true)
	if err != nil {
		log.Fatal(err)
	}

	if err := writeCRLs("testdata/crl_cert_revoked_expired.pem", [][]byte{crl_RevokedCert_expired}); err != nil {
		log.Fatal(err)
	}

	crl_irlvtRevokedCert, err := GenerateCRL(irlvtCert, irlvtKey, crlProp_revokedCert, false)
	if err != nil {
		log.Fatal(err)
	}

	crlProp_empty := []pkix.RevokedCertificate{
		{
			SerialNumber:   big.NewInt(1),
			RevocationTime: time.Now(),
		},
	}

	crl_InterCA_Empty, err := GenerateCRL(caCert, caKey, crlProp_empty, false)
	if err != nil {
		log.Fatal(err)
	}

	if err := writeCRLs("testdata/crl_inter_empty.pem", [][]byte{crl_InterCA_Empty}); err != nil {
		log.Fatal(err)
	}

	crlProp_RevokedInterCA := []pkix.RevokedCertificate{
		{
			SerialNumber:   caCert.SerialNumber,
			RevocationTime: time.Now(),
		},
	}

	crl_revokedInterCA, err := GenerateCRL(rootCert, rootKey, crlProp_RevokedInterCA, false)
	if err != nil {
		log.Fatal(err)
	}

	crl_Root_Empty, err := GenerateCRL(rootCert, rootKey, crlProp_empty, false)
	if err != nil {
		log.Fatal(err)
	}

	if err := writeCRLs("testdata/crl_root_empty.pem", [][]byte{crl_Root_Empty}); err != nil {
		log.Fatal(err)
	}

	if err := writeCRLs("testdata/crl_chain_all_empty.pem", [][]byte{crl_InterCA_Empty, crl_Root_Empty}); err != nil {
		log.Fatal(err)
	}

	if err := writeCRLs("testdata/crl_chain_cert_revoked.pem", [][]byte{crl_Root_Empty, crl_RevokedCert}); err != nil {
		log.Fatal(err)
	}

	if err := writeCRLs("testdata/crl_chain_inter_ca_cert_revoked.pem", [][]byte{crl_revokedInterCA, crl_InterCA_Empty}); err != nil {
		log.Fatal(err)
	}

	if err := writeCRLs("testdata/crl_chain_irlvt_cert_revoked.pem", [][]byte{crl_InterCA_Empty, crl_irlvtRevokedCert}); err != nil {
		log.Fatal(err)
	}

}
