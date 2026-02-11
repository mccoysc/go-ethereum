//go:build ignore
// +build ignore

package main

import (
"crypto/x509"
"encoding/asn1"
"encoding/pem"
"fmt"
"io/ioutil"
"os"
)

// OID for SGX Quote extension in RA-TLS certificate
var oidSGXQuote = asn1.ObjectIdentifier{1, 2, 840, 113741, 1, 13, 1}

func main() {
// Read certificate file
certPEM, err := ioutil.ReadFile("/tmp/test-ratls-cert.cert")
if err != nil {
fmt.Fprintf(os.Stderr, "Failed to read cert: %v\n", err)
os.Exit(1)
}

// Decode PEM
block, _ := pem.Decode(certPEM)
if block == nil {
fmt.Fprintf(os.Stderr, "Failed to decode PEM\n")
os.Exit(1)
}

// Parse certificate
cert, err := x509.ParseCertificate(block.Bytes)
if err != nil {
fmt.Fprintf(os.Stderr, "Failed to parse cert: %v\n", err)
os.Exit(1)
}

// Extract Quote from extensions
var quoteBytes []byte
for _, ext := range cert.Extensions {
if ext.Id.Equal(oidSGXQuote) {
// The value is ASN.1 encoded OCTET STRING
// Decode it to get the actual quote
if _, err := asn1.Unmarshal(ext.Value, &quoteBytes); err != nil {
fmt.Fprintf(os.Stderr, "Failed to unmarshal quote: %v\n", err)
os.Exit(1)
}
break
}
}

if quoteBytes == nil {
fmt.Fprintf(os.Stderr, "No Quote extension found in certificate\n")
os.Exit(1)
}

fmt.Printf("Quote size: %d bytes\n", len(quoteBytes))

// Output as Go byte array
fmt.Printf("\nGo byte array format for test_env.sh:\n")
fmt.Printf("REAL_QUOTE_HEX=\"")
for i := 0; i < len(quoteBytes); i++ {
fmt.Printf("%02x", quoteBytes[i])
}
fmt.Printf("\"\n")

// Write to file
err = ioutil.WriteFile("/tmp/real_quote.bin", quoteBytes, 0644)
if err != nil {
fmt.Fprintf(os.Stderr, "Failed to write quote: %v\n", err)
os.Exit(1)
}
fmt.Printf("\nQuote written to /tmp/real_quote.bin\n")

// Show first few bytes
fmt.Printf("\nFirst 100 bytes (hex):\n")
for i := 0; i < 100 && i < len(quoteBytes); i++ {
if i > 0 && i%16 == 0 {
fmt.Printf("\n")
}
fmt.Printf("%02x ", quoteBytes[i])
}
fmt.Printf("\n")
}
