/*
  certificates.go - A firmware uploader for the WiFi101 module.
  Copyright (c) 2015 Arduino LLC.  All right reserved.

  This library is free software; you can redistribute it and/or
  modify it under the terms of the GNU Lesser General Public
  License as published by the Free Software Foundation; either
  version 2.1 of the License, or (at your option) any later version.

  This library is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
  Lesser General Public License for more details.

  You should have received a copy of the GNU Lesser General Public
  License along with this library; if not, write to the Free Software
  Foundation, Inc., 51 Franklin St, Fifth Floor, Boston, MA  02110-1301  USA
*/
package certificates

import (
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"io/ioutil"
	"log"
	"path"
	"path/filepath"
	"time"
)

var START_PATTERN = []byte{0x11, 0xF1, 0x12, 0xF2, 0x13, 0xF3, 0x14, 0xF4, 0x15, 0xF5, 0x16, 0xF6, 0x17, 0xF7, 0x18, 0xF8}

type CertEntry []byte

func Convert(directory string, addresses []string) ([]byte, error) {
	var entryBytes []byte
	var numCerts int = 0

	if directory != "" {
		cerFiles, err := filepath.Glob(path.Join(directory, "*.cer"))
		if err != nil {
			return nil, err
		}

		for _, cerFile := range cerFiles {
			cerEntry, err := EntryForFile(cerFile)

			if err != nil {
				log.Printf("Converting '%v' failed, skipping: %v\n", cerFile, err)
			} else {
				entryBytes = append(entryBytes, cerEntry...)
				numCerts++
			}
		}
	}

	for _, address := range addresses {
		cerEntry, err := EntryForAddress(address)

		if err != nil {
			log.Printf("Converting address '%v' failed, skipping: %v\n", address, err)
		} else {
			entryBytes = append(entryBytes, cerEntry...)
			numCerts++
		}
	}

	numCertsBytes := uint32ToBytes(numCerts)

	flashData := START_PATTERN
	flashData = append(flashData, numCertsBytes...)
	flashData = append(flashData, entryBytes...)

	return flashData, nil
}

func ConvertCertEntries(entries []CertEntry) []byte {
	numCertsBytes := uint32ToBytes(len(entries))

	flashData := START_PATTERN
	flashData = append(flashData, numCertsBytes...)
	for _, entry := range entries {
		flashData = append(flashData, entry...)
	}

	return flashData
}

func EntryForFile(file string) (b CertEntry, err error) {
	cerData, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	certs, err := x509.ParseCertificates(cerData)
	if err != nil {
		return nil, err
	}

	if len(certs) < 1 {
		return nil, errors.New("No certificates in file")
	}

	cert := certs[0]

	return entryForCert(cert)
}

func EntryForAddress(address string) (b CertEntry, err error) {
	config := &tls.Config{
		InsecureSkipVerify: true,
	}

	conn, err := tls.Dial("tcp", address, config)
	if err != nil {
		return nil, err
	}

	if err := conn.Handshake(); err != nil {
		return nil, err
	}

	peerCerts := conn.ConnectionState().PeerCertificates

	if len(peerCerts) == 0 {
		return nil, errors.New("No peer certificates")
	}

	rootCert := peerCerts[len(peerCerts)-1]

	conn.Close()

	return entryForCert(rootCert)
}

/* Write Root Certificate to flash. The entry is ordered as follows:
-	SHA1_DIGEST_SIZE	--> NameSHA1 of the Root certificate.
-	uint16				--> N_SIZE (Byte count for the RSA modulus N).
-	uint16				--> E_SIZE (Byte count for the RSA public exponent E).
-	START_DATE			--> Start date of the root certificate(20 bytes).
-	EXPIRATION_DATE		--> Expiration date of the root certificate(20 bytes).
-	N_SIZE				--> RSA Modulus N.
-	E_SIZE				--> RSA Public exponent.
*/

func entryForCert(cert *x509.Certificate) (b CertEntry, err error) {
	nameSHA1Bytes, err := calculateNameSha1(*cert)
	if err != nil {
		return nil, err
	}

	notBeforeBytes, err := convertTime(cert.NotBefore)
	if err != nil {
		return nil, err
	}

	notAfterBytes, err := convertTime(cert.NotAfter)
	if err != nil {
		return nil, err
	}

	rsaPublicKey := *cert.PublicKey.(*rsa.PublicKey)

	rsaModulusNBytes := getModulusN(rsaPublicKey)
	rsaPublicExponentBytes := getPublicExponent(rsaPublicKey)

	rsaModulusNLenBytes := uint16ToBytes(len(rsaModulusNBytes))
	rsaPublicExponentLenBytes := uint16ToBytes(len(rsaPublicExponentBytes))

	typeBytes := []byte{0x01, 0x00, 0x00, 0x00}

	b = append(b, nameSHA1Bytes...)
	b = append(b, notBeforeBytes...)
	b = append(b, notAfterBytes...)
	b = append(b, typeBytes...)
	b = append(b, rsaModulusNLenBytes...)
	b = append(b, rsaPublicExponentLenBytes...)
	b = append(b, rsaModulusNBytes...)
	b = append(b, rsaPublicExponentBytes...)
	for (len(b) & 3) != 0 {
		b = append(b, 0xff) // padding
	}

	return
}

func uint16ToBytes(i int) (b []byte) {
	b = make([]byte, 2)

	binary.LittleEndian.PutUint16(b, uint16(i))

	return
}

func uint32ToBytes(i int) (b []byte) {
	b = make([]byte, 4)

	binary.LittleEndian.PutUint32(b, uint32(i))

	return
}

func calculateNameSha1(cert x509.Certificate) (b []byte, err error) {
	nameSha1 := sha1.New()

	var subjectDistinguishedNameSequence pkix.RDNSequence

	if _, err = asn1.Unmarshal(cert.RawSubject, &subjectDistinguishedNameSequence); err != nil {
		return nil, err
	}

	for _, dn := range subjectDistinguishedNameSequence {
		nameSha1.Write([]byte(dn[0].Value.(string)))
	}

	b = nameSha1.Sum(nil)

	return
}

func getModulusN(publicKey rsa.PublicKey) []byte {
	return publicKey.N.Bytes()
}

func getPublicExponent(publicKey rsa.PublicKey) (b []byte) {
	b = make([]byte, 4)

	binary.BigEndian.PutUint32(b, uint32(publicKey.E))

	// strip leading zeros
	for b[0] == 0 {
		b = b[1:]
	}

	return
}

func convertTime(time time.Time) (b []byte, err error) {
	b = []byte{
		byte(time.Year() & 0xff),
		byte((time.Year() >> 8) & 0xff),
		byte(time.Month()),
		byte(time.Day()),
		byte(time.Hour()),
		byte(time.Minute()),
		byte(time.Second()),
		0xcc,
	}

	return
}
