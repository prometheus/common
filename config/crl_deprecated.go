// Copyright 2016 The Prometheus Authors
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

// This file is built for It provides functionalities to parse raw CRLs,
// check their validity and verify their signatures against provided
// certificates (CAs). It also allows checking the revocation status of the
// provided certificates.

// It is built for Go versions before 1.19 as there are deprecated
// functionality related to CRL handling.

//go:build !go1.19
// +build !go1.19

package config

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"time"
)

// Parse all CRLs and return a slice of valid CRLs.
func parseCRLs(rawCRL []byte, cAs []*x509.Certificate) ([]*pkix.CertificateList, error) {
	var crls []*pkix.CertificateList
	for p, r := pem.Decode(rawCRL); p != nil; p, r = pem.Decode(r) {
		if p.Type != "X509 CRL" {
			return nil, fmt.Errorf("unable to decode raw certificate revocation list")
		}
		crl, err := x509.ParseCRL(p.Bytes)
		if err != nil {
			return nil, err
		}

		// Check CRL exipry status.
		if crl.TBSCertList.NextUpdate.Before(time.Now()) {
			return nil, fmt.Errorf("certificate revocation list is outdated")
		}

		// Check each CRL is signed by any CA, if not, ignore the CRL.
		// Otherwise, append to the valid slice of CRL.
		for _, ca := range cAs {
			err = ca.CheckCRLSignature(crl)
			if err == nil {
				crls = append(crls, crl)
				break
			}
		}
	}
	return crls, nil
}

func validRevocationStatus(cAs []*x509.Certificate, cRLs []*pkix.CertificateList) error {
	for _, cert := range cAs {
		for _, crl := range cRLs {
			for _, revokedCertificate := range crl.TBSCertList.RevokedCertificates {
				if revokedCertificate.SerialNumber.Cmp(cert.SerialNumber) == 0 {
					return fmt.Errorf("certificate was revoked")
				}
			}
		}
	}
	return nil
}
