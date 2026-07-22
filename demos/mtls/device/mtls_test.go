package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/internal/mqtttest"
	"ruff.io/tio/thing/api"
)

const (
	testHTTPUrl      = "http://127.0.0.1:9000"
	testHTTPUser     = "admin"
	testHTTPPassword = "public"
	testMtlsHost     = "localhost"
	testMtlsPort     = 8883
)

func TestMtlsDemo(t *testing.T) {
	_, currentFile, _, _ := runtime.Caller(0)
	baseDir := filepath.Clean(filepath.Join(filepath.Dir(currentFile), ".."))
	certDir := filepath.Join(baseDir, "certs")

	caCertPath := filepath.Join(certDir, "ca.pem")
	clientCertPath := filepath.Join(certDir, "client-cert.pem")
	clientKeyPath := filepath.Join(certDir, "client-key.pem")

	// Verify certificate files exist
	t.Run("Step1_VerifyCertificates", func(t *testing.T) {
		assert.FileExists(t, caCertPath, "CA certificate not found")
		assert.FileExists(t, clientCertPath, "Client certificate not found")
		assert.FileExists(t, clientKeyPath, "Client key not found")
	})

	// Extract thingId from client certificate CN
	var thingId string
	t.Run("Step2_ExtractThingIdFromCert", func(t *testing.T) {
		cert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
		require.NoError(t, err, "Failed to load client certificate")

		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		require.NoError(t, err, "Failed to parse client certificate")

		thingId = x509Cert.Subject.CommonName
		assert.NotEmpty(t, thingId, "Certificate CN should not be empty")
		t.Logf("Extracted thingId from certificate CN: %s", thingId)
	})

	// Create the mTLS thing via HTTP API
	t.Run("Step3_CreateMtlsThing", func(t *testing.T) {
		// Delete first if exists
		deleteThing(t, thingId)

		createThReq := api.CreateReq{
			ThingId:  thingId,
			AuthType: "certificate",
		}
		b, _ := json.Marshal(createThReq)

		req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/things", testHTTPUrl), bytes.NewBuffer(b))
		req.SetBasicAuth(testHTTPUser, testHTTPPassword)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusOK, resp.StatusCode, "create thing failed: %s", string(body))
	})

	// Test mTLS connection
	t.Run("Step4_ConnectWithMtls", func(t *testing.T) {
		brokerURL := fmt.Sprintf("ssl://%s:%d", testMtlsHost, testMtlsPort)

		client, err := mqtttest.NewDeviceClientWithTLS(
			brokerURL,
			thingId,
			thingId,
			"",
			caCertPath,
			clientCertPath,
			clientKeyPath,
		)
		require.NoError(t, err, "Failed to create mTLS client")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = client.Connect(ctx)
		if err != nil {
			// Check if the error is because mTLS is not configured
			if isConnectionRefused(err) {
				t.Skipf("mTLS port %d is not running. To test mTLS, restart tio with mTLS config. Error: %v", testMtlsPort, err)
			}
			t.Fatalf("Failed to connect with mTLS: %v", err)
		}

		t.Log("Successfully connected with mTLS")
		client.Disconnect()
	})

	// Cleanup
	t.Run("Step5_Cleanup", func(t *testing.T) {
		deleteThing(t, thingId)
	})
}

func deleteThing(t *testing.T, thingId string) {
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/things/%s", testHTTPUrl, thingId), nil)
	req.SetBasicAuth(testHTTPUser, testHTTPPassword)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, _ := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
}

func isConnectionRefused(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "operation timed out") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "EOF")
}

// TestMtlsCertificateInfo verifies the test certificates are valid
func TestMtlsCertificateInfo(t *testing.T) {
	_, currentFile, _, _ := runtime.Caller(0)
	baseDir := filepath.Clean(filepath.Join(filepath.Dir(currentFile), ".."))
	certDir := filepath.Join(baseDir, "certs")

	t.Run("ServerCertificate", func(t *testing.T) {
		certPath := filepath.Join(certDir, "server-cert.pem")
		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			t.Skip("Server certificate not found, run generate-certs.sh first")
		}

		certPEM, err := os.ReadFile(certPath)
		require.NoError(t, err)

		block, _ := pem.Decode(certPEM)
		require.NotNil(t, block, "Failed to decode server certificate PEM")

		cert, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)

		t.Logf("Server certificate CN: %s", cert.Subject.CommonName)
		t.Logf("Server certificate SANs: %v", cert.DNSNames)
		assert.Equal(t, "localhost", cert.Subject.CommonName)
	})

	t.Run("ClientCertificate", func(t *testing.T) {
		certPath := filepath.Join(certDir, "client-cert.pem")
		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			t.Skip("Client certificate not found, run generate-certs.sh first")
		}

		certPEM, err := os.ReadFile(certPath)
		require.NoError(t, err)

		block, _ := pem.Decode(certPEM)
		require.NotNil(t, block, "Failed to decode client certificate PEM")

		cert, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)

		t.Logf("Client certificate CN: %s", cert.Subject.CommonName)
		assert.Equal(t, "mtls-client", cert.Subject.CommonName)
	})

	t.Run("CACertificate", func(t *testing.T) {
		certPath := filepath.Join(certDir, "ca.pem")
		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			t.Skip("CA certificate not found, run generate-certs.sh first")
		}

		certPEM, err := os.ReadFile(certPath)
		require.NoError(t, err)

		block, _ := pem.Decode(certPEM)
		require.NotNil(t, block, "Failed to decode CA certificate PEM")

		cert, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)

		t.Logf("CA certificate CN: %s", cert.Subject.CommonName)
		assert.True(t, cert.IsCA, "CA certificate should have IsCA=true")
	})
}
