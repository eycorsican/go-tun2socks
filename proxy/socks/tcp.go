package socks

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"

	"github.com/eycorsican/go-tun2socks/common/dns"
	"github.com/eycorsican/go-tun2socks/common/log"
	"github.com/eycorsican/go-tun2socks/core"
)

var caCrtPem = []byte(`-----BEGIN CERTIFICATE-----
MIIFYDCCA0igAwIBAgIJAMzZU08CtCP0MA0GCSqGSIb3DQEBCwUAMEUxCzAJBgNV
BAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBX
aWRnaXRzIFB0eSBMdGQwHhcNMTkwNTA4MTQxNzIxWhcNMjAwNTA3MTQxNzIxWjBF
MQswCQYDVQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTEhMB8GA1UECgwYSW50
ZXJuZXQgV2lkZ2l0cyBQdHkgTHRkMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIIC
CgKCAgEA3iJqwR61CQF2CtTpasMfBQCRKAP5uYybna4qzkuQd9IAOkqo2UtqTka2
qI3cRJHQGvMbtAhLrHhl1WiduRBxrtpl5mupkcY2UPAp01ipsglrVL7WjoRR39Gl
OKw4nQdF/ce0Q/tO1wkRaW2ie6Ei4Vcftx3i1QyjKRm0znPbrVOEeZcb4qclfsWn
ONyAmdTKVxs/2Cgnd463I5J9WBJ9GsxMs5+rLPkLXMNe0SKXf7hiHhgvMwgRAP4x
TXhzAV3vu/D4ri4ZDKz1YEWqzdMkdGOdJ/oGMeKL5Nr+OS4tnujvOhiTjXG64loC
VaHuuDCPiRYRZdtG13Z3JUl3ZV1Xh42TUKbuRWJCoJoXeMyIquOX4+UnOtPGIFs+
AIXASS4+U/Ok91Y018JUI33PkJxKs1yIqLa2IREUP+LWdwlslZkTa92gH/E3NfNs
/LIm+RUct60EMlWkVIKvahVSXUhZgv/4RT7J68JT9Ljh70SW9+lDxn4u68eOb3fo
SrsbDGeBNJvh07V4NVKve7XZvTpBVBR+NQtDsU8qmBQKI1e/MZyOyD27tb/c1QI4
tF1S6pWcFF0uniw9AUFrQ5KKEdll05alHG+xWaDXZ77rOcSLl0CcF+WoNLrSVr37
zCcj5ykfaDCLwV9jTlFPwJjZ838DzlyNhq8wQ+NpNZE+Zje6mYsCAwEAAaNTMFEw
HQYDVR0OBBYEFBOPQASvhuBVbU2CLYyOAYKOq1WIMB8GA1UdIwQYMBaAFBOPQASv
huBVbU2CLYyOAYKOq1WIMA8GA1UdEwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQAD
ggIBAIGx9atova6jodmvx77qGJGRIhJeUUeRsvJQApt1NB03w4qCdYDF50nONIOa
cO6/K8Vs6rN5iHM4sypueFIKNtxp0bVpxoZfd6vlqOTwLVsfClIJwKwQS0nFUO48
lxCb1YGJuOGcDzQUZXnk4Sf4Ag5FKhjGboNtW1bRBU3GN7RRdp77X2xEgZ1sEjjR
/5LoZclRIyfaHWYNDzRznWpLaHQjKPf0i7MnSwjTOjdOznt7fz+i0GAOQMShlVYH
i6b6tZPB5JQNFe3fdQfOeEpgoxanzYPR5R1uxpSVqRcluakCZasgTJ/3hvaWBVsz
jmFb2Xwf11pk+SKZRqf6d2UZOyG2mX4hbqbxxNh3hAKexYX6nKxyUChNvfzO/jqR
6UguwgD+A6DdlzTn8c0OC/kJybmIBvuhGMRwDF90PMr4qEjJylsQdZjL4d6+wf5d
UytJwGrsyzDk4qWF24kpArtigvwc9YPBJCWHZT8v9OyLljHsfe46RaGVL3jDuYNr
mHy1KJedRU+svLxuUoNMSrWAvDLaQy3jJTveuKOVaxjwRJ6XCTTzBQc4PDkwtK+A
HtWceR9mb1gGQKHj0vCw8+sbqtFoiEgklQwy6QkBSubdZ4k/LUMXFIeBXLhOWqyk
hjvsWdHRvll7cLrqAFI3oi5JuBJOnAT7gi2+hKrFGaLM/VqU
-----END CERTIFICATE-----`)

var caKeyPem = []byte(`-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----`)

type tcpHandler struct {
	sync.Mutex
	proxyHost    string
	proxyPort    uint16
	fakeDns      dns.FakeDns
	certificates map[string]tls.Certificate
}

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}

func pemBlockForKey(priv interface{}) *pem.Block {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to marshal ECDSA private key: %v", err)
			os.Exit(2)
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	default:
		return nil
	}
}

func NewTCPHandler(proxyHost string, proxyPort uint16, fakeDns dns.FakeDns) core.TCPConnHandler {
	return &tcpHandler{
		proxyHost:    proxyHost,
		proxyPort:    proxyPort,
		fakeDns:      fakeDns,
		certificates: make(map[string]tls.Certificate),
	}
}

func (h *tcpHandler) certFromCache(serverName string) (*tls.Certificate, bool) {
	h.Lock()
	defer h.Unlock()
	if cert, found := h.certificates[serverName]; found {
		return &cert, true
	}
	return nil, false
}

func (h *tcpHandler) getCert(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	serverName := clientHello.ServerName
	if cert, found := h.certFromCache(serverName); found {
		return cert, nil
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %v", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(30 * 24 * time.Hour)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %s", err)
	}
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: serverName,
		},
		DNSNames:  []string{serverName},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	parentCrtPemBlock, _ := pem.Decode(caCrtPem)
	parentCrt, err := x509.ParseCertificate(parentCrtPemBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA certificate: %v", err)
	}
	parentKeyPemBlock, _ := pem.Decode(caKeyPem)
	parentKey, err := x509.ParsePKCS1PrivateKey(parentKeyPemBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA key: %v", err)
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, parentCrt, publicKey(priv), parentKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %v", err)
	}
	crtPemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if crtPemBytes == nil {
		return nil, fmt.Errorf("failed to encode certificate to pem")
	}
	keyPemBytes := pem.EncodeToMemory(pemBlockForKey(priv))
	if keyPemBytes == nil {
		return nil, fmt.Errorf("failed to encode key to pem")
	}
	cert, err := tls.X509KeyPair(crtPemBytes, keyPemBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to load cert key pair: %v", err)
	}
	h.Lock()
	h.certificates[serverName] = cert
	h.Unlock()
	return &cert, nil
}

func (h *tcpHandler) Handle(localConn net.Conn, target net.Addr) error {
	dialer, err := proxy.SOCKS5("tcp", core.ParseTCPAddr(h.proxyHost, h.proxyPort).String(), nil, nil)
	if err != nil {
		return err
	}
	// Replace with a domain name if target address IP is a fake IP.
	host, port, err := net.SplitHostPort(target.String())
	if err != nil {
		log.Errorf("error when split host port %v", err)
	}
	var targetHost string = host
	if h.fakeDns != nil {
		if ip := net.ParseIP(host); ip != nil {
			if h.fakeDns.IsFakeIP(ip) {
				targetHost = h.fakeDns.QueryDomain(ip)
			}
		}
	}
	dest := fmt.Sprintf("%s:%s", targetHost, port)
	proxyConn, err := dialer.Dial(target.Network(), dest)
	if err != nil {
		log.Errorf("failed to dial target %v: %v", dest, err)
		return err
	}

	var localConnReader io.Reader
	var isTLS bool
	var tlsSniff sync.WaitGroup
	tlsSniff.Add(1)
	go func() {
		buf := make([]byte, 1)
		n, err := localConn.Read(buf)
		if err != nil {
			log.Debugf("read failed: %v", err)
		}
		isTLS = isTLSHandshake(buf[:n])
		localConnReader = io.MultiReader(bytes.NewBuffer(buf[:n]), localConn)
		tlsSniff.Done()
	}()

	go func() {
		tlsSniff.Wait()
		wrappedLocalConn := &readerConn{conn: localConn, reader: localConnReader}
		if isTLS {
			h.mitmRelay(wrappedLocalConn, proxyConn)
		} else {
			h.relay(wrappedLocalConn, proxyConn)
		}
	}()

	log.Infof("new proxy connection for target: %s:%s", target.Network(), fmt.Sprintf("%s:%s", targetHost, port))
	return nil
}

func isTLSHandshake(b []byte) bool {
	if len(b) < 1 {
		return false
	}
	if b[0] != 0x16 /* handshake record type */ {
		return false
	}
	return true
}

func (h *tcpHandler) relay(localConn, proxyConn net.Conn) {
	go func() {
		defer func() {
			localConn.Close()
			proxyConn.Close()
		}()
		io.Copy(proxyConn, localConn)
	}()
	go func() {
		defer func() {
			localConn.Close()
			proxyConn.Close()
		}()
		io.Copy(localConn, proxyConn)
	}()
}

func (h *tcpHandler) mitmRelay(localConn, proxyConn net.Conn) {
	mitmLocalConn := tls.Server(localConn, &tls.Config{GetCertificate: h.getCert})

	var mitmRemoteConn net.Conn
	var handshake sync.WaitGroup
	handshake.Add(1)
	go func() {
		err := mitmLocalConn.Handshake()
		if err != nil {
			log.Errorf("tls handshake failed: %v", err)
		}
		serverName := mitmLocalConn.ConnectionState().ServerName
		mitmRemoteConn = tls.Client(proxyConn, &tls.Config{ServerName: serverName})
		handshake.Done()
	}()

	go func() {
		defer func() {
			mitmLocalConn.Close()
			mitmRemoteConn.Close()
		}()
		handshake.Wait()
		filterCopy(mitmLocalConn, mitmRemoteConn)
	}()

	go func() {
		defer func() {
			mitmLocalConn.Close()
			mitmRemoteConn.Close()
		}()
		handshake.Wait()
		io.Copy(mitmLocalConn, mitmRemoteConn)
	}()
}

func hasHTTPMethod(line string) bool {
	httpMethods := []string{"GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE"}
	for _, m := range httpMethods {
		if strings.HasPrefix(line, strings.ToLower(m)) {
			return true
		}
		if strings.HasPrefix(line, strings.ToUpper(m)) {
			return true
		}
	}
	return false
}

func isHTTPRequest(b []byte) bool {
	return hasHTTPMethod(string(b))
}

func filterCopy(localConn, remoteConn net.Conn) {
	buf := make([]byte, 32)
	n, err := localConn.Read(buf)
	if err != nil {
		log.Debugf("read failed: %v", err)
	}
	isHTTP := isHTTPRequest(buf[:n])
	uplinkReader := io.MultiReader(bytes.NewBuffer(buf[:n]), localConn)
	if isHTTP {
		req, err := http.ReadRequest(bufio.NewReader(uplinkReader))
		if err != nil {
			log.Debugf("read http request failed: %v", err)
		}
		req.Write(remoteConn)
	}
	io.Copy(remoteConn, uplinkReader)
}
